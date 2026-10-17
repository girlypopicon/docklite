package docker

import (
	"strings"
	"testing"
)

func TestEngineFor(t *testing.T) {
	for _, in := range []string{"", "postgres", "POSTGRES", "mariadb", "mongodb", "redis"} {
		if _, ok := EngineFor(in); !ok {
			t.Errorf("%q should be a known type", in)
		}
	}
	for _, in := range []string{"mysql", "sqlite", "oracle", "postgres; drop"} {
		if _, ok := EngineFor(in); ok {
			t.Errorf("%q must not be accepted as a container engine", in)
		}
	}
	if e, _ := EngineFor(""); e.Type != "postgres" {
		t.Error("an empty type means postgres: every database made before there were other types is one")
	}
}

func TestEnginesAreSafeByDefault(t *testing.T) {
	for _, e := range EngineTypes() {
		if e.Type != "postgres" && !e.Loopback {
			t.Errorf("%s must listen on this machine only by default", e.Label)
		}
		if e.Port == 0 || e.Image == "" {
			t.Errorf("%s needs an image and a port", e.Label)
		}
	}
	// Postgres keeps its old behaviour so existing setups don't change underneath people.
	if pg, _ := EngineFor("postgres"); pg.Loopback || pg.DataDir != "" {
		t.Error("postgres creation must stay as it was")
	}
}

func TestEngineEnvAndCmd(t *testing.T) {
	maria, _ := EngineFor("mariadb")
	env := strings.Join(maria.ContainerEnv("shop", "app", "pw1", "rootpw"), " ")
	for _, want := range []string{"MARIADB_DATABASE=shop", "MARIADB_USER=app", "MARIADB_PASSWORD=pw1", "MARIADB_ROOT_PASSWORD=rootpw"} {
		if !strings.Contains(env, want) {
			t.Errorf("mariadb env missing %s: %s", want, env)
		}
	}
	mongo, _ := EngineFor("mongodb")
	env = strings.Join(mongo.ContainerEnv("shop", "app", "pw1", ""), " ")
	if !strings.Contains(env, "MONGO_INITDB_ROOT_USERNAME=app") || !strings.Contains(env, "MONGO_INITDB_ROOT_PASSWORD=pw1") {
		t.Errorf("mongodb env: %s", env)
	}
	redis, _ := EngineFor("redis")
	cmd := redis.ContainerCmd("pw1")
	if strings.Join(cmd, " ") != "redis-server --requirepass pw1 --appendonly yes" {
		t.Errorf("redis must require its password and persist data: %v", cmd)
	}
	if mongo.ContainerCmd("x") != nil || maria.ContainerCmd("x") != nil {
		t.Error("only redis needs a command line")
	}
}

func TestConnectionURLs(t *testing.T) {
	cases := map[string]string{
		"postgres": "postgresql://u:p@db:5432/shop",
		"mariadb":  "mysql://u:p@db:3306/shop",
		"mongodb":  "mongodb://u:p@db:27017/shop?authSource=admin",
		"redis":    "redis://:p@db:6379",
	}
	for typ, want := range cases {
		e, _ := EngineFor(typ)
		if got := e.ConnectionURL("db", e.Port, "shop", "u", "p"); got != want {
			t.Errorf("%s: got %s want %s", typ, got, want)
		}
	}
}
