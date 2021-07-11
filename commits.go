package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	. "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing"
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

func init() {
	Commits = make(map[string]*Commit)
	Branches = make(map[string]string)
	Tags = make(map[string]string)
}

// Return the first commit, the root of the tree
func dfs(r *git.Repository, hash plumbing.Hash, chash plumbing.Hash, bName string) []string {
	hstr := hash.String()
	cstr := chash.String()

	// If commit hash is already encountered, then we are reaching the commit from a different
	// span. So, add the child hash to the commit and stop the traversal.
	if commit, ok := Commits[hstr]; ok {
		if cstr != NIL_COMMIT {
			// Append the child hash only if it is known
			Commits[hstr].Children = append(Commits[hstr].Children, cstr)
		}
		if len(Commits[hstr].Children) > 1 {
			fmt.Printf("Commit already exists (split): %s <- %s, %v\n", hash, cstr, commit)
		} else {
			fmt.Printf("Commit already exists: %s <- %s, %v\n", hash, cstr, commit)
		}

		// Also, append the branch
		_, bok := commit.Branches[bName]
		if bok {
			commit.Branches[bName] += 1
		} else {
			commit.Branches[bName] = 1
		}
		// return the empty string as this traversal stopped without reaching the root
		return []string{}
	}

	// Otherwise, store the commit object
	c, err := r.CommitObject(hash)
	CheckIfError(err)

	Commits[hstr] = &Commit{
		Hash:       hstr,
		CommitWhen: c.Committer.When,
		AuthorWhen: c.Author.When,
		Children:   make([]string, 0),
		Parents:    make([]string, 0),
		Tags:		make([]string, 0),
		Branches:   make(map[string]int32),
	}
	// Update the branch. Need to revisit the entire branch logic later
	Commits[hstr].Branches[bName] = 1
	// Store the child hash ONLY if it is not ""
	if cstr != NIL_COMMIT {
		Commits[hstr].Children = append(Commits[hstr].Children, cstr)
	}

	fmt.Printf("Storing commit: %s <- %s, %v\n", hstr, cstr, Commits[hstr])

	// Update the parents
	root := []string{}
	if len(c.ParentHashes) == 0 {
		// Reached the first commit (root), nothing more to traverse
		fmt.Printf("Reached the first commit %s\n", hstr)
		root = append(root, hstr)
	} else {
		numParents := len(c.ParentHashes)
		if numParents > 1 {
			fmt.Printf("Merge commit %s from %d commits\n", hstr, numParents)
		}
		for _, phash := range c.ParentHashes {
			Commits[hstr].Parents = append(Commits[hstr].Parents, phash.String())
			rets := dfs(r, phash, hash, bName)
			root = append(root, rets...)
		}
	}

	return root
}


func bfs(hash string, span uint32, monoclock time.Time) {
	commit := Commits[hash]
	commit.Span = span
	if commit.CommitWhen.Before(monoclock) {
		// set the InferredTime to mono clock + 1 millisecond
		monoclock.Add(time.Millisecond)
		commit.InferredTime = monoclock
	} else {
		commit.InferredTime = commit.CommitWhen
		monoclock = commit.InferredTime
	}

	fmt.Printf("%s: Populated commit %v\n", hash, commit)

	numChildren := len(commit.Children)
	if numChildren == 0 {
		// Reached the last commit, nothing more to traverse
		fmt.Printf("%s: Reached the end commit with %v\n", hash, commit)
		return
	}

	for _, chash := range commit.Children {
		cc := Commits[chash]
		// skip the child commit if it has already by visited from a prior root
		if cc.Span != SPAN_NOT_SET {
			fmt.Printf("%s: Child commit is already visited, skipping %v\n", chash, cc)
			continue
		}

		if len(cc.Parents) == 1 {
			// If this commit is a split commit, i.e has more than one child commit, then
			// increment the span for the child commit
			ccspan := span
			if numChildren > 1 {
				nextSpan++; ccspan = nextSpan
				commit.ChildSpan = append(commit.ChildSpan, ccspan)
				fmt.Printf("%s: Updated commit %v\n", hash, commit)
				cc.ParentSpan = append(cc.ParentSpan, span)
			}
			// Continue traversing the child commit as it has only one parent
			bfs(chash, ccspan, monoclock)
		}
		if len(cc.Parents) > 1 {
			fmt.Printf("%s: Child commit is a merge commit with parents %v\n", chash, cc.Parents)
			// The child happens to be a merge commit, the child will have multiple parent commits
			// Traverse only those child commit(s) that have all their parents visited
			// Visited can be verified by looking into the span or the inferred time.
			parentsNotVisited := []string{}
			for _, phash := range cc.Parents {
				if Commits[phash].Span == SPAN_NOT_SET {
					parentsNotVisited = append(parentsNotVisited, phash)
				}
			}
			if len(parentsNotVisited) > 0 {
				fmt.Printf("%s: Skipping merge commit becuase parents %v not traversed\n", chash, parentsNotVisited)
				continue
			}

			// Reached here because all parents have been visited

			// compute spans below...
			// Start with a new span
			nextSpan++; ccspan := nextSpan
			// Set the child spans for the parents and parent spans for this commit
			for _, ccphash := range cc.Parents {
				Commits[ccphash].ChildSpan = append(Commits[ccphash].ChildSpan, ccspan)
				fmt.Printf("%s: Updated commit %v\n", ccphash, Commits[ccphash])
				cc.ParentSpan = append(cc.ParentSpan, Commits[ccphash].Span)
			}

			// compute monoclock below...
			// The monoclock for the child has to be after the monoclock of all the parents
			ccmonoclock := monoclock
			for _, ccphash := range cc.Parents {
				if ccmonoclock.Before(Commits[ccphash].InferredTime) {
					ccmonoclock = Commits[ccphash].InferredTime
				}
			}

			fmt.Printf("%s: Traversing merge commit with span %d and reference clock %v\n", chash, ccspan, ccmonoclock)
			bfs(chash, ccspan, ccmonoclock)
		}
	}
}