package coreapi_test

import (
	"context"
	"strings"
	"testing"
)

func TestMutablePath(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// get self /ipns path
	keys, err := api.Key().List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !keys[0].Path().Mutable() {
		t.Error("expected self /ipns path to be mutable")
	}

	blk, err := api.Block().Put(ctx, strings.NewReader(`foo`))
	if err != nil {
		t.Error(err)
	}

	if blk.Mutable() {
		t.Error("expected /ipld path to be immutable")
	}
}

func TestPathRemainder(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.ParsePath(obj.String() + "/foo/bar")
	if err != nil {
		t.Error(err)
	}

	rp1, err := api.ResolvePath(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if rp1.Remainder() != "foo/bar" {
		t.Error("expected to get path remainder")
	}
}

func TestEmptyPathRemainder(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	if obj.Remainder() != "" {
		t.Error("expected the resolved path to not have a remainder")
	}

	p1, err := api.ParsePath(obj.String())
	if err != nil {
		t.Error(err)
	}

	rp1, err := api.ResolvePath(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if rp1.Remainder() != "" {
		t.Error("expected the resolved path to not have a remainder")
	}
}

func TestInvalidPathRemainder(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	obj, err := api.Dag().Put(ctx, strings.NewReader(`{"foo": {"bar": "baz"}}`))
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.ParsePath(obj.String() + "/bar/baz")
	if err != nil {
		t.Error(err)
	}

	_, err = api.ResolvePath(ctx, p1)
	if err == nil || err.Error() != "no such link found" {
		t.Fatalf("unexpected error: %s", err)
	}
}
