package filecache

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
)

func TestCacherFunctions(t *testing.T) {
	cacher, err := NewCacher(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatalf("Failed to create cacher: %v", err)
	}

	bucket := Bucket{
		name: "test",
		ttl:  10 * time.Second,
	}

	keys := []string{"key1", "key2", "key3"}

	type valStruct = struct {
		Name string
	}

	values := []*valStruct{
		{
			Name: "value1",
		},
		{
			Name: "value2",
		},
		{
			Name: "value3",
		},
	}

	for i, key := range keys {
		err = cacher.Set(bucket, key, values[i])
		if err != nil {
			t.Fatalf("Failed to set the value: %v", err)
		}
	}

	allVals, err := GetAll[*valStruct](cacher, bucket)
	if err != nil {
		t.Fatalf("Failed to get all values: %v", err)
	}

	if len(allVals) != len(keys) {
		t.Fatalf("Failed to get all values: expected %d, got %d", len(keys), len(allVals))
	}

	spew.Dump(allVals)
}

func TestCacherSetAndGet(t *testing.T) {
	cacher, err := NewCacher(filepath.Join(t.TempDir(), "cache"))

	bucket := Bucket{
		name: "test",
		ttl:  4 * time.Second,
	}
	key := "key"
	value := struct {
		Name string
	}{
		Name: "value",
	}
	// Add "key" -> value to the bucket, with a TTL of 4 seconds
	err = cacher.Set(bucket, key, value)
	if err != nil {
		t.Fatalf("Failed to set the value: %v", err)
	}

	var out struct {
		Name string
	}
	// Get the value of "key" from the bucket, it shouldn't be expired
	found, err := cacher.Get(bucket, key, &out)
	if err != nil {
		t.Errorf("Failed to get the value: %v", err)
	}
	if !found || !assert.Equal(t, value, out) {
		t.Errorf("Failed to get the correct value. Expected %v, got %v", value, out)
	}

	spew.Dump(out)

	time.Sleep(3 * time.Second)

	// Get the value of "key" from the bucket again, it shouldn't be expired
	found, err = cacher.Get(bucket, key, &out)
	if !found {
		t.Errorf("Failed to get the value")
	}
	if !found || out != value {
		t.Errorf("Failed to get the correct value. Expected %v, got %v", value, out)
	}

	spew.Dump(out)

	// Spin up a goroutine to set "key2" -> value2 to the bucket, with a TTL of 1 second
	// cacher should be thread-safe
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		key2 := "key2"
		value2 := struct {
			Name string
		}{
			Name: "value2",
		}
		var out2 struct {
			Name string
		}
		err = cacher.Set(bucket, key2, value2)
		if err != nil {
			t.Errorf("Failed to set the value: %v", err)
		}

		found, err = cacher.Get(bucket, key2, &out2)
		if err != nil {
			t.Errorf("Failed to get the value: %v", err)
		}

		if !found || !assert.Equal(t, value2, out2) {
			t.Errorf("Failed to get the correct value. Expected %v, got %v", value2, out2)
		}

		_ = cacher.Delete(bucket, key2)

		spew.Dump(out2)

	}()

	time.Sleep(2 * time.Second)

	// Get the value of "key" from the bucket, it should be expired
	found, _ = cacher.Get(bucket, key, &out)
	if found {
		t.Errorf("Failed to delete the value")
		spew.Dump(out)
	}

	wg.Wait()

}

func TestGetPermFresh(t *testing.T) {
	cacher, err := NewCacher(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatalf("Failed to create cacher: %v", err)
	}

	bucket := NewPermanentBucket("perm")
	value := struct{ Name string }{Name: "value"}

	if err := cacher.SetPerm(bucket, "key", value); err != nil {
		t.Fatalf("SetPerm failed: %v", err)
	}

	var out struct{ Name string }

	// A generous maxAge serves the just-written entry from cache.
	found, err := cacher.GetPermFresh(bucket, "key", &out, time.Hour)
	if err != nil {
		t.Fatalf("GetPermFresh failed: %v", err)
	}
	if !found || out != value {
		t.Fatalf("expected fresh hit, got found=%v out=%v", found, out)
	}

	// A tiny maxAge treats the entry as stale -> miss (so the caller hits the network).
	out = struct{ Name string }{}
	found, err = cacher.GetPermFresh(bucket, "key", &out, time.Nanosecond)
	if err != nil {
		t.Fatalf("GetPermFresh (stale) failed: %v", err)
	}
	if found {
		t.Fatalf("expected stale miss, got hit out=%v", out)
	}

	// A missing key is a miss regardless of maxAge.
	found, _ = cacher.GetPermFresh(bucket, "absent", &out, time.Hour)
	if found {
		t.Fatalf("expected miss for absent key")
	}

	// maxAge <= 0 disables the age check (behaves like GetPerm).
	found, err = cacher.GetPermFresh(bucket, "key", &out, 0)
	if err != nil {
		t.Fatalf("GetPermFresh (no age) failed: %v", err)
	}
	if !found || out != value {
		t.Fatalf("expected hit with maxAge=0, got found=%v out=%v", found, out)
	}
}
