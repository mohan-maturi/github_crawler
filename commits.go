package main

import (
	"time"
)

// Commit within a repo
type Commit struct {
	Hash         string
	InferredTime time.Time
	CommitWhen   time.Time
	AuthorWhen   time.Time
	// The commits that precede this commit
	Parents []string
	// The commits that follow this commit. If this is the last commit in the chain
	// the value will be empty.
	Children []string
	// The tag assigned to this commit
	Tags []string
	// Branch heads pointing to this commit
	BranchHeads []string
	// Is initial commit
	InitialCommit bool
	// Span identifier used to traverse a path
	Span uint32
	// Span identifier of the parent split commit. Populated only for the split commits
	ParentSpan []uint32
	// Span identifier of the child merge commit. Populated only for the merge commits
	ChildSpan []uint32

	// TDOD: To be deprecated
	Branches map[string]int32
}