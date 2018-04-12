/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package internalversion

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	core "k8s.io/kubernetes/pkg/apis/core"
)

// VolumeSnapshotLister helps list VolumeSnapshots.
type VolumeSnapshotLister interface {
	// List lists all VolumeSnapshots in the indexer.
	List(selector labels.Selector) (ret []*core.VolumeSnapshot, err error)
	// VolumeSnapshots returns an object that can list and get VolumeSnapshots.
	VolumeSnapshots(namespace string) VolumeSnapshotNamespaceLister
	VolumeSnapshotListerExpansion
}

// volumeSnapshotLister implements the VolumeSnapshotLister interface.
type volumeSnapshotLister struct {
	indexer cache.Indexer
}

// NewVolumeSnapshotLister returns a new VolumeSnapshotLister.
func NewVolumeSnapshotLister(indexer cache.Indexer) VolumeSnapshotLister {
	return &volumeSnapshotLister{indexer: indexer}
}

// List lists all VolumeSnapshots in the indexer.
func (s *volumeSnapshotLister) List(selector labels.Selector) (ret []*core.VolumeSnapshot, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*core.VolumeSnapshot))
	})
	return ret, err
}

// VolumeSnapshots returns an object that can list and get VolumeSnapshots.
func (s *volumeSnapshotLister) VolumeSnapshots(namespace string) VolumeSnapshotNamespaceLister {
	return volumeSnapshotNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// VolumeSnapshotNamespaceLister helps list and get VolumeSnapshots.
type VolumeSnapshotNamespaceLister interface {
	// List lists all VolumeSnapshots in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*core.VolumeSnapshot, err error)
	// Get retrieves the VolumeSnapshot from the indexer for a given namespace and name.
	Get(name string) (*core.VolumeSnapshot, error)
	VolumeSnapshotNamespaceListerExpansion
}

// volumeSnapshotNamespaceLister implements the VolumeSnapshotNamespaceLister
// interface.
type volumeSnapshotNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all VolumeSnapshots in the indexer for a given namespace.
func (s volumeSnapshotNamespaceLister) List(selector labels.Selector) (ret []*core.VolumeSnapshot, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*core.VolumeSnapshot))
	})
	return ret, err
}

// Get retrieves the VolumeSnapshot from the indexer for a given namespace and name.
func (s volumeSnapshotNamespaceLister) Get(name string) (*core.VolumeSnapshot, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(core.Resource("volumesnapshot"), name)
	}
	return obj.(*core.VolumeSnapshot), nil
}
