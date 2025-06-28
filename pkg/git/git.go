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
	cryptossh "golang.org/x/crypto/ssh"
)

type Repository struct {
	URI             string
	Repo            *gogit.Repository
	Tree            *gogit.Worktree
	AuthMethod      ssh.AuthMethod
	CurrentRevision string
	CurrentHash     string
}

func Clone(uri, authMethod, username, password, fileBase string) (*Repository, error) {
	var err error
	var auth ssh.AuthMethod
	var sshAgent *ssh.PublicKeysCallback
	opts := &gogit.CloneOptions{
		URL:   uri,
		Depth: 1,
	}
	switch authMethod {
	case "sshAgent":
		sshAgent, err = ssh.NewSSHAgentAuth(username)
		auth = sshAgent
		if err != nil {
			return nil, fmt.Errorf("sshAgent auth setup %v: %v", uri, err)
		}
	case "sshPrivateKey":
		auth, err = ssh.NewPublicKeys(username, []byte(password), "")
		if err != nil {
			return nil, fmt.Errorf("sshPrivateKey auth setup %v: %v", uri, err)
		}
	}
	opts.Auth = auth
	repo, err := gogit.PlainClone(fileBase, false, opts)
	if err != nil {
		// The local sshAgent may hold multiple keys.
		// It's possible that the first key tried
		// was authenticated but not authorized to the repo.
		// If this is the case we should try each key available
		// in turn before giving up.
		success := false
		if authMethod == "sshAgent" && sshAgent != nil {
			var signers []cryptossh.Signer
			signers, err = sshAgent.Callback()
			if err != nil {
				return nil, fmt.Errorf("sshAgent auth failed, and found no signers in sshAgent %v: %v", uri, err)
			}
			for _, signer := range signers {
				auth = &ssh.PublicKeys{
					User:   username,
					Signer: signer,
				}
				opts.Auth = auth
				repo, err = gogit.PlainClone(fileBase, false, opts)
				if err == nil {
					success = true
					break // Successfully cloned with a different key
				}
			}
		}
		if !success {
			return nil, fmt.Errorf("cloning %v, authMethod: %v: %v", uri, authMethod, err)
		}
	}
	tree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("creating worktree: %v", err)
	}
	r := &Repository{
		URI:             uri,
		Repo:            repo,
		Tree:            tree,
		AuthMethod:      auth,
		CurrentRevision: "HEAD",
		CurrentHash:     "",
	}
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
		err = r.fetchOrigin(mirrorRemoteBranchRefSpec)
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

func (r *Repository) fetchOrigin(refSpecStr string) error {
	remote, err := r.Repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("create remote: %v", err)
	}

	var refSpecs []config.RefSpec
	if refSpecStr != "" {
		refSpecs = []config.RefSpec{config.RefSpec(refSpecStr)}
	}

	if err = remote.Fetch(&gogit.FetchOptions{
		RefSpecs: refSpecs,
		Auth:     r.AuthMethod,
	}); err != nil {
		if err == gogit.NoErrAlreadyUpToDate {
			fmt.Print("refs already up to date")
		} else {
			return fmt.Errorf("fetch origin failed: %v", err)
		}
	}

	return nil
}
