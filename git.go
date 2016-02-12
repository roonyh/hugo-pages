package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	git "github.com/libgit2/git2go"
)

// Clone clones a repo and also updates the repos submodules.
func Clone(url string, path string, branch string) (*git.Repository, error) {
	co := &git.CloneOptions{
		CheckoutOpts: &git.CheckoutOpts{
			Strategy: git.CheckoutForce | git.CheckoutUpdateSubmodules,
		},
		CheckoutBranch: branch,
	}

	repo, err := git.Clone(url, path, co)
	if err != nil {
		return repo, err
	}

	// At the moments the submodules are not updated recursively.
	repo.Submodules.Foreach(func(sub *git.Submodule, name string) int {
		err = sub.Update(true, &git.SubmoduleUpdateOptions{
			CheckoutOpts: &git.CheckoutOpts{
				Strategy: git.CheckoutForce,
			},
			FetchOptions:          &git.FetchOptions{},
			CloneCheckoutStrategy: git.CheckoutForce,
		})

		return 0
	})

	return repo, nil
}

// Checkout creates a new repository in the generated `public` folder and
// creates a new commit with the generated stuff
func Checkout(pathToPublic string) (*git.Repository, error) {
	repo, err := git.InitRepository(pathToPublic, false)
	if err != nil {
		return nil, err
	}

	idx, err := repo.Index()
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(pathToPublic, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}

		if fi.IsDir() {
			if path[len(pathToPublic):] == ".git" {
				return filepath.SkipDir // Skip entire .git directory
			}
			return nil
		}

		err = idx.AddByPath(path[len(pathToPublic):])
		if err != nil {
			panic(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	treeID, err := idx.WriteTree()
	if err != nil {
		return nil, err
	}

	err = idx.Write()
	if err != nil {
		return nil, err
	}

	tree, err := repo.LookupTree(treeID)
	if err != nil {
		return nil, err
	}

	signature := &git.Signature{
		Name:  "Hugopages",
		Email: "contact@hugopages.io",
		When:  time.Now(),
	}

	message := "Publish to gh-pages"
	_, err = repo.CreateCommit("refs/heads/gh-pages", signature, signature, message, tree)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// Push pushes the `gh-pages` branch to the given url
func Push(url string, repo *git.Repository) error {
	remote, err := repo.Remotes.Create("by-hgpages-service", url)
	if err != nil {
		return err
	}

	err = remote.Push([]string{":refs/heads/gh-pages"}, nil)
	if err != nil {
		return err
	}

	err = remote.Push([]string{"refs/heads/gh-pages"}, nil)
	if err != nil {
		return err
	}

	return nil
}
