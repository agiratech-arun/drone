package server

import (
	"time"

	"github.com/agiratech-arun/drone/model"
	"github.com/agiratech-arun/drone/remote"
	"github.com/agiratech-arun/drone/store"
)

// Syncer synces the user repository and permissions.
type Syncer interface {
	Sync(user *model.User) error
}

type syncer struct {
	remote  remote.Remote
	store   store.Store
	perms   model.PermStore
	limiter model.Limiter
}

func (s *syncer) Sync(user *model.User) error {
	unix := time.Now().Unix() - (3601) // force immediate expiration. note 1 hour expiration is hard coded at the moment
	repos, err := s.remote.Repos(user)
	if err != nil {
		return err
	}

	if s.limiter != nil {
		repos = s.limiter.LimitRepos(user, repos)
	}

	var perms []*model.Perm
	for _, repo := range repos {
		perm := model.Perm{
			UserID: user.ID,
			Repo:   repo.FullName,
			Pull:   true,
			Synced: unix,
		}
		if repo.Perm != nil {
			perm.Push = repo.Perm.Push
			perm.Admin = repo.Perm.Admin
		}
		perms = append(perms, &perm)
	}

	err = s.store.RepoBatch(repos)
	if err != nil {
		return err
	}

	err = s.store.PermBatch(perms)
	if err != nil {
		return err
	}

	// this is here as a precaution. I want to make sure that if an api
	// call to the version control system fails and (for some reason) returns
	// an empty list, we don't wipe out the user repository permissions.
	//
	// the side-effect of this code is that a user with 1 repository whose
	// access is removed will still display in the feed, but they will not
	// be able to access the actual repository data.
	if len(repos) == 0 {
		return nil
	}

	return s.perms.PermFlush(user, unix)
}
