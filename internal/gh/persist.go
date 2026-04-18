package gh

import (
	"fmt"

	pb "cxcdn/internal/cache"
)

func init() {
	pb.RegisterMarshaler(func(key string, value interface{}, expiration int64) (*pb.CacheEntry, error) {
		tree, ok := value.(*GitHubTree)
		if !ok {
			return nil, errNotMyType
		}
		return &pb.CacheEntry{
			Key:          key,
			ExpirationNs: expiration,
			Value: &pb.CacheEntry_GithubTree{GithubTree: &pb.GitHubTree{
				Sha:       tree.Sha,
				Items:     ghTreeItemsToProto(tree.Tree),
				Truncated: tree.Truncated,
			}},
		}, nil
	})

	pb.RegisterUnmarshaler(func(entry *pb.CacheEntry) (interface{}, error) {
		switch v := entry.Value.(type) {
		case *pb.CacheEntry_GithubTree:
			return &GitHubTree{
				Sha:       v.GithubTree.Sha,
				Tree:      protoToGHTreeItems(v.GithubTree.Items),
				Truncated: v.GithubTree.Truncated,
			}, nil
		default:
			return nil, errNotMyType
		}
	})
}

var errNotMyType = fmt.Errorf("not my type")

func ghTreeItemsToProto(items []GitHubTreeItem) []*pb.GitHubTreeItem {
	result := make([]*pb.GitHubTreeItem, len(items))
	for i, item := range items {
		result[i] = &pb.GitHubTreeItem{
			Path: item.Path,
			Mode: item.Mode,
			Type: item.Type,
			Sha:  item.Sha,
			Size: item.Size,
		}
	}
	return result
}

func protoToGHTreeItems(items []*pb.GitHubTreeItem) []GitHubTreeItem {
	result := make([]GitHubTreeItem, len(items))
	for i, item := range items {
		result[i] = GitHubTreeItem{
			Path: item.Path,
			Mode: item.Mode,
			Type: item.Type,
			Sha:  item.Sha,
			Size: item.Size,
		}
	}
	return result
}
