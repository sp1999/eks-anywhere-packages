package bundle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/aws/eks-anywhere-packages/api/v1alpha1"
	"github.com/aws/eks-anywhere-packages/pkg/testutil"
)

// TODO(ivyostosh): Add a TestLatestBundle test that validates the latest bundle name is properly formatted.

func TestDownloadBundle(t *testing.T) {
	t.Parallel()

	baseRef := "example.com/org"
	discovery := testutil.NewFakeDiscoveryWithDefaults()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithFileData(t, "../../api/testdata/bundle_one.yaml")

		bm := NewBundleManager(logr.Discard(), discovery, puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		bundle, err := bm.DownloadBundle(ctx, ref)

		if err != nil {
			t.Fatalf("expected no error, got: %s", err)
		}

		if bundle == nil {
			t.Errorf("expected bundle to be non-nil")
		}

		if bundle != nil && len(bundle.Spec.Packages) != 3 {
			t.Errorf("expected three packages to be defined, found %d",
				len(bundle.Spec.Packages))
		}
		if bundle.Spec.Packages[0].Name != "Test" {
			t.Errorf("expected first package name to be \"Test\", got: %q",
				bundle.Spec.Packages[0].Name)
		}
		if bundle.Spec.Packages[1].Name != "Flux" {
			t.Errorf("expected second package name to be \"Flux\", got: %q",
				bundle.Spec.Packages[1].Name)
		}
		if bundle.Spec.Packages[2].Name != "Harbor" {
			t.Errorf("expected third package name to be \"Harbor\", got: %q",
				bundle.Spec.Packages[2].Name)
		}
	})

	t.Run("handles pull errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithError(fmt.Errorf("test error"))

		bm := NewBundleManager(logr.Discard(), discovery, puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("errors on empty repsonses", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		puller := testutil.NewMockPuller()
		puller.WithData([]byte(""))

		bm := NewBundleManager(logr.Discard(), discovery, puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("handles YAML unmarshaling errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		// stub oras.Pull
		puller := testutil.NewMockPuller()
		puller.WithData([]byte("invalid yaml"))

		bm := NewBundleManager(logr.Discard(), discovery, puller)

		kubeVersion := "v1-21"
		tag := "latest"
		ref := fmt.Sprintf("%s:%s-%s", baseRef, kubeVersion, tag)

		_, err := bm.DownloadBundle(ctx, ref)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
		// The k8s YAML library converts everything to JSON, so the error we'll
		// get will be a JSON one.
		if !strings.Contains(err.Error(), "JSON") {
			t.Errorf("expected YAML-related error, got: %s", err)
		}
	})
}

func TestKubeVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		expected := "v1.21"
		if ver, _ := kubeVersion("v1.21-42"); ver != expected {
			t.Errorf("expected %q, got %q", expected, ver)
		}
	})
}

func TestPackageVersion(t *testing.T) {
	t.Parallel()

	t.Run("golden path", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})

	t.Run("minor version+", func(t *testing.T) {
		t.Parallel()

		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		got, err := bm.apiVersion()
		if err != nil {
			t.Fatalf("expected no error, got %s", err)
		}
		expected := "v1-21"
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	noBundles := []api.PackageBundle{}

	t.Run("marks state active", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, true, noBundles)) {
			assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
		}
	})

	t.Run("marks state inactive", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, false, noBundles)) {
			assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (inactive)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateInactive,
			},
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, false, noBundles)) {
			assert.Equal(t, api.PackageBundleStateInactive, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (active)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, true, noBundles)) {
			assert.Equal(t, api.PackageBundleStateActive, bundle.Status.State)
		}
	})

	t.Run("marks state upgrade available", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml")
		bundle.Status.State = api.PackageBundleStateActive
		allBundles := []api.PackageBundle{
			*bundle,
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, true, allBundles)) {
			assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
		}
	})

	t.Run("leaves state as-is (upgrade available)", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml")
		bundle.Status.State = api.PackageBundleStateUpgradeAvailable
		allBundles := []api.PackageBundle{
			*bundle,
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}
		bm := NewBundleManager(logr.Discard(), discovery, puller)

		if assert.True(t, bm.Update(bundle, true, allBundles)) {
			assert.Equal(t, api.PackageBundleStateUpgradeAvailable, bundle.Status.State)
		}
	})
}

func TestSortBundleNewestFirst(t *testing.T) {
	t.Run("it sorts newest version first", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		allBundles := []api.PackageBundle{
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		bm := NewBundleManager(logr.Discard(), discovery, puller)
		_ = bm.Update(bundle, true, allBundles)
		if assert.Greater(t, len(allBundles), 1) {
			assert.Equal(t, "v1-21-1002", allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
		}
	})

	t.Run("invalid names go to the end", func(t *testing.T) {
		discovery := testutil.NewFakeDiscoveryWithDefaults()
		puller := testutil.NewMockPuller()
		bundle := &api.PackageBundle{
			Status: api.PackageBundleStatus{
				State: api.PackageBundleStateActive,
			},
		}
		allBundles := []api.PackageBundle{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1-16-1003",
				},
				Status: api.PackageBundleStatus{
					State: api.PackageBundleStateInactive,
				},
			},
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_one.yaml"),
			*api.MustPackageBundleFromFilename(t, "../../api/testdata/bundle_two.yaml"),
		}

		bm := NewBundleManager(logr.Discard(), discovery, puller)
		_ = bm.Update(bundle, true, allBundles)
		if assert.Greater(t, len(allBundles), 2) {
			assert.Equal(t, "v1-21-1002", allBundles[0].Name)
			assert.Equal(t, "v1-21-1001", allBundles[1].Name)
			assert.Equal(t, "v1-16-1003", allBundles[2].Name)

		}
	})
}
