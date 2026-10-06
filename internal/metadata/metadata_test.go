package metadata

import "testing"

func TestRoundTrip(t *testing.T) {
	orig := Metadata{AppName: "MyApp", AppVersion: "1.2.3", MainExe: "myapp.exe", Publisher: "ACME"}
	data, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != orig {
		t.Fatalf("got %+v, want %+v", got, orig)
	}
}

func TestValidate(t *testing.T) {
	valid := Metadata{AppName: "MyApp", AppVersion: "1.0.0", MainExe: "myapp.exe"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}
	if err := (Metadata{}).Validate(); err == nil {
		t.Fatal("empty metadata accepted")
	}
	if err := (Metadata{AppName: "  ", AppVersion: "1.0.0", MainExe: "x.exe"}).Validate(); err == nil {
		t.Fatal("whitespace-only name accepted")
	}
}
