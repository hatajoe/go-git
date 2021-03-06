// Package revlist provides support to access the ancestors of commits, in a
// similar way as the git-rev-list command.
package revlist

import (
	"fmt"
	"io"

	"srcd.works/go-git.v4/plumbing"
	"srcd.works/go-git.v4/plumbing/object"
	"srcd.works/go-git.v4/plumbing/storer"
)

// Objects applies a complementary set. It gets all the hashes from all
// the reachable objects from the given objects. Ignore param are object hashes
// that we want to ignore on the result. All that objects must be accessible
// from the object storer.
func Objects(
	s storer.EncodedObjectStorer,
	objects []plumbing.Hash,
	ignore []plumbing.Hash) ([]plumbing.Hash, error) {

	seen := hashListToSet(ignore)
	result := make(map[plumbing.Hash]bool)

	walkerFunc := func(h plumbing.Hash) {
		if !seen[h] {
			result[h] = true
			seen[h] = true
		}
	}

	for _, h := range objects {
		if err := processObject(s, h, seen, walkerFunc); err != nil {
			return nil, err
		}
	}

	return hashSetToList(result), nil
}

// processObject obtains the object using the hash an process it depending of its type
func processObject(
	s storer.EncodedObjectStorer,
	h plumbing.Hash,
	seen map[plumbing.Hash]bool,
	walkerFunc func(h plumbing.Hash),
) error {
	o, err := s.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return err
	}

	do, err := object.DecodeObject(s, o)
	if err != nil {
		return err
	}

	switch do := do.(type) {
	case *object.Commit:
		return reachableObjects(do, seen, walkerFunc)
	case *object.Tree:
		return iterateCommitTrees(seen, do, walkerFunc)
	case *object.Tag:
		walkerFunc(do.Hash)
		return processObject(s, do.Target, seen, walkerFunc)
	case *object.Blob:
		walkerFunc(do.Hash)
	default:
		return fmt.Errorf("object type not valid: %s. "+
			"Object reference: %s", o.Type(), o.Hash())
	}

	return nil
}

// reachableObjects returns, using the callback function, all the reachable
// objects from the specified commit. To avoid to iterate over seen commits,
// if a commit hash is into the 'seen' set, we will not iterate all his trees
// and blobs objects.
func reachableObjects(
	commit *object.Commit,
	seen map[plumbing.Hash]bool,
	cb func(h plumbing.Hash)) error {
	return object.WalkCommitHistory(commit, func(commit *object.Commit) error {
		if seen[commit.Hash] {
			return nil
		}

		cb(commit.Hash)

		tree, err := commit.Tree()
		if err != nil {
			return err
		}

		return iterateCommitTrees(seen, tree, cb)
	})
}

// iterateCommitTrees iterate all reachable trees from the given commit
func iterateCommitTrees(
	seen map[plumbing.Hash]bool,
	tree *object.Tree,
	cb func(h plumbing.Hash)) error {
	if seen[tree.Hash] {
		return nil
	}

	cb(tree.Hash)

	treeWalker := object.NewTreeWalker(tree, true)

	for {
		_, e, err := treeWalker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if seen[e.Hash] {
			continue
		}

		cb(e.Hash)
	}

	return nil
}

func hashSetToList(hashes map[plumbing.Hash]bool) []plumbing.Hash {
	var result []plumbing.Hash
	for key := range hashes {
		result = append(result, key)
	}

	return result
}

func hashListToSet(hashes []plumbing.Hash) map[plumbing.Hash]bool {
	result := make(map[plumbing.Hash]bool)
	for _, h := range hashes {
		result[h] = true
	}

	return result
}
