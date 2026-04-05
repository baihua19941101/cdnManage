package provider

import "fmt"

type Registry struct {
	objectStorage map[Type]ObjectStorageProvider
	cdn           map[Type]CDNProvider
}

func NewRegistry() *Registry {
	return &Registry{
		objectStorage: make(map[Type]ObjectStorageProvider),
		cdn:           make(map[Type]CDNProvider),
	}
}

func (r *Registry) RegisterObjectStorage(p ObjectStorageProvider) error {
	if p == nil {
		return fmt.Errorf("object storage provider is nil")
	}
	if !IsKnownType(p.Type()) || p.Type() == TypeUnknown {
		return fmt.Errorf("object storage provider type %q is not supported", p.Type())
	}
	r.objectStorage[p.Type()] = p
	return nil
}

func (r *Registry) RegisterCDN(p CDNProvider) error {
	if p == nil {
		return fmt.Errorf("cdn provider is nil")
	}
	if !IsKnownType(p.Type()) || p.Type() == TypeUnknown {
		return fmt.Errorf("cdn provider type %q is not supported", p.Type())
	}
	r.cdn[p.Type()] = p
	return nil
}

func (r *Registry) ObjectStorage(t Type) (ObjectStorageProvider, error) {
	provider, ok := r.objectStorage[t]
	if !ok {
		return nil, NewError(t, ServiceObjectStorage, "resolve_provider", ErrCodeUnsupportedProvider, "object storage provider is not registered", false, nil)
	}
	return provider, nil
}

func (r *Registry) CDN(t Type) (CDNProvider, error) {
	provider, ok := r.cdn[t]
	if !ok {
		return nil, NewError(t, ServiceCDN, "resolve_provider", ErrCodeUnsupportedProvider, "cdn provider is not registered", false, nil)
	}
	return provider, nil
}
