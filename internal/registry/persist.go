package registry

import (
	"fmt"

	pb "cxcdn/internal/cache"
)

func init() {
	pb.RegisterMarshaler(func(key string, value interface{}, expiration int64) (*pb.CacheEntry, error) {
		pkg, ok := value.(*NpmPackage)
		if !ok {
			return nil, errNotMyType
		}
		return &pb.CacheEntry{
			Key:          key,
			ExpirationNs: expiration,
			Value: &pb.CacheEntry_NpmPackage{NpmPackage: &pb.NpmPackage{
				Name:     pkg.Name,
				Versions: npmVersionsToProto(pkg.Versions),
				DistTags: pkg.DistTags,
			}},
		}, nil
	})

	pb.RegisterUnmarshaler(func(entry *pb.CacheEntry) (interface{}, error) {
		switch v := entry.Value.(type) {
		case *pb.CacheEntry_NpmPackage:
			return &NpmPackage{
				Name:     v.NpmPackage.Name,
				Versions: protoToNpmVersions(v.NpmPackage.Versions),
				DistTags: v.NpmPackage.DistTags,
			}, nil
		default:
			return nil, errNotMyType
		}
	})
}

var errNotMyType = fmt.Errorf("not my type")

func npmVersionsToProto(versions map[string]NpmVersion) map[string]*pb.NpmVersion {
	result := make(map[string]*pb.NpmVersion, len(versions))
	for k, v := range versions {
		result[k] = &pb.NpmVersion{
			Name:    v.Name,
			Version: v.Version,
			Tarball: v.Dist.Tarball,
		}
	}
	return result
}

func protoToNpmVersions(versions map[string]*pb.NpmVersion) map[string]NpmVersion {
	result := make(map[string]NpmVersion, len(versions))
	for k, v := range versions {
		result[k] = NpmVersion{
			Name:    v.Name,
			Version: v.Version,
			Dist: struct {
				Tarball string `json:"tarball"`
			}{
				Tarball: v.Tarball,
			},
		}
	}
	return result
}
