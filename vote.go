package main

import (
	"context"
	"errors"

	"github.com/Jguer/aur"
	"github.com/Jguer/votar/pkg/vote"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/query"
)

type ErrAURVote struct {
	inner   error
	pkgName string
}

func (e *ErrAURVote) Error() string {
	return gotext.Get("Unable to handle package vote for: %s. err: %s", e.pkgName, e.inner.Error())
}

func handlePackageVote(ctx context.Context,
	targets []string, aurClient *aur.Client,
	voteClient *vote.Client, splitN int, upvote bool,
) error {
	infos, err := query.AURInfoPrint(ctx, aurClient, targets, splitN)
	if err != nil {
		return err
	}

	for _, info := range infos {
		var err error
		if upvote {
			err = voteClient.Vote(ctx, info.PackageBase)
		} else {
			err = voteClient.Unvote(ctx, info.PackageBase)
		}

		if err != nil {
			if errors.Is(err, vote.ErrNoCredentials) {
				return errors.New(
					gotext.Get("%s: please set AUR_USERNAME and AUR_PASSWORD environment variables for voting",
						err.Error()))
			}

			return &ErrAURVote{inner: err, pkgName: info.Name}
		}
	}

	return nil
}
