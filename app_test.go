package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStartupStoresRuntimeContext(t *testing.T) {
	app := NewApp()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startup(ctx)

	if app.ctx != ctx {
		t.Fatal("startup did not retain the runtime context")
	}
}

func TestGetAppInfo(t *testing.T) {
	info := NewApp().GetAppInfo()
	if info.Name != "Zowe Worship" || info.Version != "0.1.0" {
		t.Fatalf("unexpected application info: %+v", info)
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"name":"Zowe Worship","version":"0.1.0"}` {
		t.Fatalf("unexpected bridge payload: %s", data)
	}
}
