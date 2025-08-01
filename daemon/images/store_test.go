package images

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/core/metadata"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins/content/local"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/v2/daemon/internal/image"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func setupTestStores(t *testing.T) (context.Context, content.Store, *imageStoreWithLease, func(t *testing.T)) {
	dir, err := os.MkdirTemp("", t.Name())
	assert.NilError(t, err)

	backend, err := image.NewFSStoreBackend(filepath.Join(dir, "images"))
	assert.NilError(t, err)
	imgStore, err := image.NewImageStore(backend, nil)
	assert.NilError(t, err)

	db, err := bolt.Open(filepath.Join(dir, "metadata.db"), 0o600, nil)
	assert.NilError(t, err)

	cs, err := local.NewStore(filepath.Join(dir, "content"))
	assert.NilError(t, err)
	mdb := metadata.NewDB(db, cs, nil)

	cleanup := func(t *testing.T) {
		assert.Check(t, db.Close())
		assert.Check(t, os.RemoveAll(dir))
	}
	ctx := namespaces.WithNamespace(context.Background(), t.Name())
	images := &imageStoreWithLease{Store: imgStore, ns: t.Name(), leases: metadata.NewLeaseManager(mdb)}

	return ctx, cs, images, cleanup
}

func TestImageDelete(t *testing.T) {
	ctx, _, images, cleanup := setupTestStores(t)
	defer cleanup(t)

	t.Run("no lease", func(t *testing.T) {
		id, err := images.Create([]byte(`{"rootFS": {}}`))
		assert.NilError(t, err)
		defer images.Delete(id)

		ls, err := images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Equal(t, len(ls), 0, ls)

		_, err = images.Delete(id)
		assert.NilError(t, err, "should not error when there is no lease")
	})

	t.Run("lease exists", func(t *testing.T) {
		id, err := images.Create([]byte(`{"rootFS": {}}`))
		assert.NilError(t, err)
		defer images.Delete(id)

		leaseID := imageKey(id.String())
		_, err = images.leases.Create(ctx, leases.WithID(leaseID))
		assert.NilError(t, err)
		defer images.leases.Delete(ctx, leases.Lease{ID: leaseID})

		ls, err := images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(len(ls), 1), ls)

		_, err = images.Delete(id)
		assert.NilError(t, err)

		ls, err = images.leases.List(ctx)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(len(ls), 0), ls)
	})
}

func TestContentStoreForPull(t *testing.T) {
	ctx, cs, imgStore, cleanup := setupTestStores(t)
	defer cleanup(t)

	csP := &contentStoreForPull{
		ContentStore: cs,
		leases:       imgStore.leases,
	}

	data := []byte(`{}`)
	desc := ocispec.Descriptor{
		Digest: digest.Canonical.FromBytes(data),
		Size:   int64(len(data)),
	}

	w, err := csP.Writer(ctx, content.WithRef(t.Name()), content.WithDescriptor(desc))
	assert.NilError(t, err)

	_, err = w.Write(data)
	assert.NilError(t, err)
	defer w.Close()

	err = w.Commit(ctx, desc.Size, desc.Digest)
	assert.NilError(t, err)

	assert.Equal(t, len(csP.digested), 1)
	assert.Check(t, is.Equal(csP.digested[0], desc.Digest))

	// Test already exists
	csP.digested = nil
	_, err = csP.Writer(ctx, content.WithRef(t.Name()), content.WithDescriptor(desc))
	assert.Check(t, cerrdefs.IsAlreadyExists(err))
	assert.Equal(t, len(csP.digested), 1)
	assert.Check(t, is.Equal(csP.digested[0], desc.Digest))
}
