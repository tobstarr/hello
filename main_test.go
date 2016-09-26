package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHandler(t *testing.T) {
	l := log.New(ioutil.Discard, "", 0)
	s := httptest.NewServer(mux(l, "v2"))
	defer s.Close()
	rsp, err := http.Get(s.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer rsp.Body.Close()
	if rsp.Status[0] != '2' {
		b, _ := ioutil.ReadAll(rsp.Body)
		t.Fatalf("got status %s but expected 2x. body=%s", rsp.Status, string(b))
	}

	var res *response
	buf := &bytes.Buffer{}
	err = json.NewDecoder(io.TeeReader(rsp.Body, buf)).Decode(&res)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct{ Has, Want interface{} }{
		{res.Version, "v2"},
		{res.Message, "Hello World!"},
	}
	for i, tc := range tests {
		if tc.Has != tc.Want {
			t.Errorf("%d: want=%#v has=%#v", i+1, tc.Want, tc.Has)
		}
	}
}
