package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"docklite-agent/internal/store"
)

// A "server" destination sends backups to another DockLite server that is connected on the Servers page, using the
// token that server made for this one. The other server keeps them in its own backups folder, under incoming/.
type hubRemote struct {
	srv    store.RemoteServer
	client *http.Client
}

func newHubRemote(srv store.RemoteServer) *hubRemote {
	return &hubRemote{srv: srv, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (h *hubRemote) ref(name string) string {
	return "server://" + strconv.FormatInt(h.srv.ID, 10) + "/" + name
}

func (h *hubRemote) nameFromRef(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, "server://"+strconv.FormatInt(h.srv.ID, 10)+"/")
	if !ok || name == "" {
		return "", fmt.Errorf("%s isn't on server %s", ref, h.srv.Name)
	}
	return name, nil
}

func (h *hubRemote) do(ctx context.Context, method, path string, q url.Values, body io.Reader, length int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, h.srv.URL+path+"?"+q.Encode(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.srv.Token)
	if body != nil {
		req.ContentLength = length
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach %s: %w", h.srv.Name, err)
	}
	if resp.StatusCode/100 != 2 {
		defer resp.Body.Close()
		var e struct {
			Error string `json:"error"`
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		_ = json.Unmarshal(b, &e)
		if e.Error == "" {
			e.Error = fmt.Sprintf("answered %d", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusNotFound {
			e.Error += " (is that server updated to a version that accepts backups?)"
		}
		return nil, fmt.Errorf("%s: %s", h.srv.Name, e.Error)
	}
	return resp, nil
}

func (h *hubRemote) Put(ctx context.Context, localPath, name string) (string, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	resp, err := h.do(ctx, http.MethodPost, "/api/backups/incoming", url.Values{"name": {name}}, &ctxReader{ctx: ctx, r: f}, st.Size())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Name  string `json:"name"`
		Bytes int64  `json:"bytes"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || out.Name == "" {
		return "", fmt.Errorf("%s didn't confirm the backup arrived", h.srv.Name)
	}
	if out.Bytes != st.Size() {
		return "", fmt.Errorf("only %d of %d bytes arrived at %s", out.Bytes, st.Size(), h.srv.Name)
	}
	return h.ref(out.Name), nil
}

func (h *hubRemote) Get(ctx context.Context, ref, localPath string) error {
	name, err := h.nameFromRef(ref)
	if err != nil {
		return err
	}
	resp, err := h.do(ctx, http.MethodGet, "/api/backups/incoming/file", url.Values{"name": {name}}, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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

func (h *hubRemote) Delete(ctx context.Context, ref string) error {
	name, err := h.nameFromRef(ref)
	if err != nil {
		return err
	}
	resp, err := h.do(ctx, http.MethodDelete, "/api/backups/incoming", url.Values{"name": {name}}, nil, 0)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (h *hubRemote) Test(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := h.do(ctx, http.MethodGet, "/api/backups/incoming", url.Values{}, nil, 0)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return "Connected to " + h.srv.Name + ": it accepts backups.", nil
}
