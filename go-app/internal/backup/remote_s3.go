package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

// An S3 destination talks to the bucket directly over HTTPS with signed requests, so no cloud library is needed and it
// works with anything that speaks the S3 protocol. Files up to 64 MB go up in one request; bigger ones in 64 MB parts,
// each retried on its own, so a dropped connection costs a part, not the whole upload.
const (
	s3PartSize     = 64 << 20
	s3SingleLimit  = 64 << 20
	s3TestObject   = ".docklite-test"
	s3DefaultRange = "us-east-1"
)

type s3Remote struct {
	cfg    DestinationConfig
	client *http.Client
	now    func() time.Time
}

func newS3(cfg DestinationConfig) *s3Remote {
	return &s3Remote{cfg: cfg, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now}
}

func (s *s3Remote) region() string {
	if s.cfg.Region != "" {
		return s.cfg.Region
	}
	return s3DefaultRange
}

func (s *s3Remote) endpoint() string {
	if s.cfg.Endpoint != "" {
		return strings.TrimRight(s.cfg.Endpoint, "/")
	}
	return "https://s3." + s.region() + ".amazonaws.com"
}

func (s *s3Remote) key(name string) string {
	p := strings.Trim(s.cfg.Prefix, "/")
	name = strings.TrimLeft(name, "/")
	if p == "" {
		return name
	}
	return p + "/" + name
}

func (s *s3Remote) ref(key string) string { return "s3://" + s.cfg.Bucket + "/" + key }

// keyFromRef is the object key a reference points at, and only if it is in this destination's bucket.
func (s *s3Remote) keyFromRef(ref string) (string, error) {
	rest, ok := strings.CutPrefix(ref, "s3://"+s.cfg.Bucket+"/")
	if !ok || rest == "" {
		return "", fmt.Errorf("%s is not in the bucket %s", ref, s.cfg.Bucket)
	}
	return rest, nil
}

func s3Escape(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && keepSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// request builds a signed request (AWS signature v4). payloadHash is the sha256 of the body, hex.
func (s *s3Remote) request(ctx context.Context, method, key string, query url.Values, body io.Reader, length int64, payloadHash string) (*http.Request, error) {
	base, err := url.Parse(s.endpoint())
	if err != nil {
		return nil, err
	}
	path := "/" + s.cfg.Bucket
	if key != "" {
		path += "/" + key
	}
	canonPath := s3Escape(path, true)
	var qs []string
	for k, vs := range query {
		for _, v := range vs {
			qs = append(qs, s3Escape(k, false)+"="+s3Escape(v, false))
		}
	}
	sort.Strings(qs)
	rawQuery := strings.Join(qs, "&")
	full := base.Scheme + "://" + base.Host + canonPath
	if rawQuery != "" {
		full += "?" + rawQuery
	}
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, err
	}
	req.ContentLength = length
	t := s.now().UTC()
	amzDate, day := t.Format("20060102T150405Z"), t.Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	host := base.Host
	canonHeaders := "host:" + host + "\nx-amz-content-sha256:" + payloadHash + "\nx-amz-date:" + amzDate + "\n"
	signed := "host;x-amz-content-sha256;x-amz-date"
	canonical := strings.Join([]string{method, canonPath, rawQuery, canonHeaders, signed, payloadHash}, "\n")
	scope := day + "/" + s.region() + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	k := hmacSHA256([]byte("AWS4"+s.cfg.SecretKey), day)
	k = hmacSHA256(k, s.region())
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(k, toSign))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", s.cfg.AccessKey, scope, signed, sig))
	return req, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// do sends a request with a small body held in memory and returns the answer's body, or an error with what the service said.
func (s *s3Remote) do(ctx context.Context, method, key string, query url.Values, body []byte) ([]byte, http.Header, error) {
	req, err := s.request(ctx, method, key, query, bytes.NewReader(body), int64(len(body)), hashBytes(body))
	if err != nil {
		return nil, nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return out, resp.Header, s3Error(resp.StatusCode, out)
	}
	return out, resp.Header, nil
}

// s3Error turns a failed answer into words a person can act on.
func s3Error(status int, body []byte) error {
	var e struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	_ = xml.Unmarshal(body, &e)
	hint := ""
	switch {
	case e.Code == "NoSuchBucket" || status == 404:
		hint = " Check the bucket name, and that it already exists."
	case e.Code == "InvalidAccessKeyId" || e.Code == "SignatureDoesNotMatch":
		hint = " Check the access key and secret key, and the region."
	case e.Code == "AccessDenied" || status == 403:
		hint = " The keys are fine but aren't allowed to write to this bucket."
	}
	if e.Code != "" {
		return fmt.Errorf("the storage service said %s: %s.%s", e.Code, strings.TrimSuffix(e.Message, "."), hint)
	}
	return fmt.Errorf("the storage service answered %d.%s", status, hint)
}

func (s *s3Remote) Put(ctx context.Context, localPath, name string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	key := s.key(name)
	if st.Size() <= s3SingleLimit {
		data := make([]byte, st.Size())
		if _, err := io.ReadFull(f, data); err != nil {
			return "", err
		}
		err := retry(ctx, func() error { _, _, err := s.do(ctx, http.MethodPut, key, nil, data); return err })
		return s.ref(key), err
	}
	return s.ref(key), s.putMultipart(ctx, f, st.Size(), key)
}

func (s *s3Remote) putMultipart(ctx context.Context, f *os.File, size int64, key string) error {
	out, _, err := s.do(ctx, http.MethodPost, key, url.Values{"uploads": {""}}, nil)
	if err != nil {
		return err
	}
	var init struct {
		UploadID string `xml:"UploadId"`
	}
	if xml.Unmarshal(out, &init) != nil || init.UploadID == "" {
		return fmt.Errorf("the storage service didn't start the upload")
	}
	abort := func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _, _ = s.do(c, http.MethodDelete, key, url.Values{"uploadId": {init.UploadID}}, nil)
	}
	var parts strings.Builder
	parts.WriteString("<CompleteMultipartUpload>")
	buf := make([]byte, s3PartSize)
	for n, off := 1, int64(0); off < size; n++ {
		if err := ctx.Err(); err != nil {
			abort()
			return err
		}
		m, err := io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			abort()
			return err
		}
		part := buf[:m]
		var etag string
		err = retry(ctx, func() error {
			_, h, err := s.do(ctx, http.MethodPut, key, url.Values{"partNumber": {fmt.Sprint(n)}, "uploadId": {init.UploadID}}, part)
			if err == nil {
				etag = h.Get("ETag")
			}
			return err
		})
		if err != nil {
			abort()
			return fmt.Errorf("part %d: %w", n, err)
		}
		fmt.Fprintf(&parts, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", n, etag)
		off += int64(m)
	}
	parts.WriteString("</CompleteMultipartUpload>")
	res, _, err := s.do(ctx, http.MethodPost, key, url.Values{"uploadId": {init.UploadID}}, []byte(parts.String()))
	if err != nil {
		abort()
		return err
	}
	if bytes.Contains(res, []byte("<Error>")) { // the service can answer 200 and still fail at the last step
		abort()
		return s3Error(500, res)
	}
	return nil
}

func (s *s3Remote) Get(ctx context.Context, ref, localPath string) error {
	key, err := s.keyFromRef(ref)
	if err != nil {
		return err
	}
	req, err := s.request(ctx, http.MethodGet, key, nil, nil, 0, hashBytes(nil))
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return s3Error(resp.StatusCode, b)
	}
	out, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, &ctxReader{ctx: ctx, r: resp.Body})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && resp.ContentLength >= 0 && n != resp.ContentLength {
		err = fmt.Errorf("the download stopped early")
	}
	if err != nil {
		_ = os.Remove(localPath)
	}
	return err
}

func (s *s3Remote) Delete(ctx context.Context, ref string) error {
	key, err := s.keyFromRef(ref)
	if err != nil {
		return err
	}
	_, _, err = s.do(ctx, http.MethodDelete, key, nil, nil)
	return err
}

func (s *s3Remote) Test(ctx context.Context) (string, error) {
	key := s.key(s3TestObject)
	if _, _, err := s.do(ctx, http.MethodPut, key, nil, []byte("docklite")); err != nil {
		return "", err
	}
	if _, _, err := s.do(ctx, http.MethodDelete, key, nil, nil); err != nil {
		return "", fmt.Errorf("it can write but not delete, so old backups could never be cleaned up: %w", err)
	}
	return "Connected to bucket " + s.cfg.Bucket + ": can write and delete.", nil
}
