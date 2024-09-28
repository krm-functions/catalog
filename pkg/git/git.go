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
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Repository struct {
	URI             string
	Repo            *gogit.Repository
	Tree            *gogit.Worktree
	CurrentRevision string
	CurrentHash     string
}

func Clone(uri, authMethod, username, password, fileBase string) (*Repository, error) {
	var err error
	opts := &gogit.CloneOptions{
		URL: uri,
	}
	if authMethod == "sshAgent" {
		var auth *ssh.PublicKeysCallback
		auth, err = ssh.NewSSHAgentAuth(username)
		if err != nil {
			return nil, fmt.Errorf("sshAgent auth setup %v: %v", uri, err)
		}
		opts.Auth = auth
	} else if authMethod == "sshPrivateKey" {
		var auth *ssh.PublicKeys
		auth, err = ssh.NewPublicKeys(username, []byte(password), "")
		if err != nil {
			return nil, fmt.Errorf("sshPrivateKey auth setup %v: %v", uri, err)
		}
		opts.Auth = auth
	}
	repo, err := gogit.PlainClone(fileBase, false, opts)
	if err != nil {
		return nil, fmt.Errorf("cloning %v, authMethod: %v: %v", uri, authMethod, err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %v", err)
	}
	r := &Repository{uri, repo, tree, "HEAD", ""}
	hash, err := r.ResolveRevision("HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolving HEAD: %v", err)
	}
	r.CurrentHash = hash
	return r, nil
}

func (r *Repository) ResolveRevision(treeishRevision string) (string, error) {
	hash, err := r.Repo.ResolveRevision(plumbing.Revision(treeishRevision))
	if err != nil {
		// Unknown ref, try remote branch
		mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", treeishRevision, treeishRevision)
		err = fetchOrigin(r.Repo, mirrorRemoteBranchRefSpec)
		if err != nil {
			return "", fmt.Errorf("fetch remote %v@%v: %v", r.URI, treeishRevision, err)
		}
		hash, err = r.Repo.ResolveRevision(plumbing.Revision(treeishRevision))
		if err != nil {
			return "", fmt.Errorf("unknown ref %v@ %v: %v", r.URI, treeishRevision, err)
		}
	}
	return hash.String(), nil
}

func (r *Repository) Checkout(treeishRevision string) (string, error) {
	if r.CurrentRevision == treeishRevision {
		return r.CurrentHash, nil // already at revision
	}

	hash, err := r.ResolveRevision(treeishRevision)
	if err != nil {
		return "", fmt.Errorf("failed to resolve ref %v@%v: %v", r.URI, treeishRevision, err)
	}
	opts := gogit.CheckoutOptions{
		Hash: plumbing.NewHash(hash),
	}
	if err := r.Tree.Checkout(&opts); err != nil {
		return "", fmt.Errorf("checkout %v@%v: %v", r.URI, treeishRevision, err)
	}

	r.CurrentRevision = treeishRevision
	r.CurrentHash = hash
	return hash, nil
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
