package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		fmt.Println(err)

		return 0
	})

	return repo, nil
}

// Checkout creates a new repository in the generated `public` folder and
// creates a new commit with the generated stuff
func Checkout(pathToPublic string) *git.Repository {
	repo, err := git.InitRepository(pathToPublic, false)
	if err != nil {
		panic(err)
	}

	idx, err := repo.Index()
	if err != nil {
		panic(err)
	}

	filepath.Walk(pathToPublic, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if fi.IsDir() {
			return nil
		}

		if strings.HasPrefix(path[len(pathToPublic):], ".git/") {
			return nil
		}

		fmt.Println(path[len(pathToPublic):])
		err = idx.AddByPath(path[len(pathToPublic):])
		if err != nil {
			panic(err)
		}
		return nil
	})

	treeID, err := idx.WriteTree()
	if err != nil {
		panic(err)
	}

	err = idx.Write()
	if err != nil {
		panic(err)
	}

	tree, err := repo.LookupTree(treeID)
	if err != nil {
		panic(err)
	}

	signature := &git.Signature{
		Name:  "hg-pages publisher",
		Email: "arunabherath@gmail.com",
		When:  time.Now(),
	}

	message := "Publish to gh-pages"
	commitID, err := repo.CreateCommit("refs/heads/gh-pages", signature, signature, message, tree)
	if err != nil {
		panic(err)
	}
	fmt.Println(commitID)
	return repo
}

// Push pushes the `gh-pages` branch to the given url
func Push(url string, repo *git.Repository) {
	remote, err := repo.Remotes.Create("by-hgpages-service", url)
	if err != nil {
		panic(err)
	}

	err = remote.Push([]string{":refs/heads/gh-pages"}, nil)
	if err != nil {
		fmt.Println(err)
	}

	err = remote.Push([]string{"refs/heads/gh-pages"}, nil)
	if err != nil {
		fmt.Println(err)
	}
}
