// Copyright 2024 Michael Vittrup Larsen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package git

import (
	"fmt"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type Repository struct {
	URI             string
	Repo            *gogit.Repository
	Tree            *gogit.Worktree
	CurrentCheckout string
}

func Clone(uri, fileBase string) (*Repository, error) {
	repo, err := gogit.PlainClone(fileBase, false, &gogit.CloneOptions{
		URL: uri,
	})
	if err != nil {
		return nil, fmt.Errorf("cloning %v: %v", uri, err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %v", err)
	}
	return &Repository{uri, repo, tree, "HEAD"}, nil
}

func (r *Repository) Checkout(treeishRevision string) error {
	if r.CurrentCheckout == treeishRevision {
		return nil // already at revision
	}
	branch := treeishRevision // FIXME
	branchRefName := plumbing.NewBranchReferenceName(branch)
	branchCoOpts := gogit.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchRefName),
	}
	if err := r.Tree.Checkout(&branchCoOpts); err != nil {
		// Local checkout failed, try remote
		mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
		err = fetchOrigin(r.Repo, mirrorRemoteBranchRefSpec)
		if err != nil {
			return fmt.Errorf("fetch remote %v @ %v: %v", r.URI, branch, err)
		}
		err = r.Tree.Checkout(&branchCoOpts)
		if err != nil {
			return fmt.Errorf("checkout %v @ %v: %v", r.URI, branch, err)
		}
	}
	r.CurrentCheckout = treeishRevision
	return nil
}

func fetchOrigin(repo *gogit.Repository, refSpecStr string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("create remote: %v", err)
	}

	var refSpecs []config.RefSpec
	if refSpecStr != "" {
		refSpecs = []config.RefSpec{config.RefSpec(refSpecStr)}
	}

	if err = remote.Fetch(&gogit.FetchOptions{
		RefSpecs: refSpecs,
	}); err != nil {
		if err == gogit.NoErrAlreadyUpToDate {
			fmt.Print("refs already up to date")
		} else {
			return fmt.Errorf("fetch origin failed: %v", err)
		}
	}

	return nil
}
