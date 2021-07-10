package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	. "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing"
)

const NIL_COMMIT string = "0000000000000000000000000000000000000000"
const SPAN_NOT_SET uint32 = 0
const START_SPAN uint32 = 1

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

var Branches map[string]string
var Tags map[string]string
var Commits map[string]*Commit

var nextSpan uint32

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
			fmt.Printf("Found split commit: %s <- %s, %v\n", hash, cstr, commit)
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

func CheckoutBranches(path string) *git.Repository {
	fmt.Printf("%s: CheckoutBranches invoked\n", time.Now().String())
	// We instantiate a new repository targeting the given path (the .git folder)
	r, err := git.PlainOpen(path)
	CheckIfError(err)

	remotes, err := r.Remotes()
	CheckIfError(err)
	remote := remotes[0]
	fmt.Printf("%s: Fetching remotes for %s\n", time.Now().String(), remote)
	r.Fetch(&git.FetchOptions{})

	// Get the branches and tags
	refBranchPrefix := "refs/heads/"
	refTagPrefix := "refs/tags/"
	refList, err := remote.List(&git.ListOptions{})
	CheckIfError(err)
	for _, ref := range refList {
		refName := ref.Name().String()
		hash := ref.Hash().String()

		if strings.HasPrefix(refName, refBranchPrefix) {
			branchName := refName[len(refBranchPrefix):]
			Branches[branchName] = hash
		} else {
			if strings.HasPrefix(refName, refTagPrefix) {
				tagName := refName[len(refTagPrefix):]
				Tags[tagName] = hash
			}
		}
	}

	// Log all the branches
	for b, c := range Branches {
		fmt.Printf("Branch %s: %s\n", b, c)
	}

	// Get the worktree to check out branches locally
	worktree, err := r.Worktree()
	CheckIfError(err)
	fmt.Println("Worktree: ", worktree)

	// checkout branches locally
	fmt.Printf("%s: Checking out Branches locally \n", time.Now().String())
	for k, v := range Branches {
		fmt.Printf("%s: %s\n", time.Now().String(), PrintMemUsage())
		fmt.Printf("%s: Checking out... %s at %s\n", time.Now().String(), k, v)
		worktree.Checkout(&git.CheckoutOptions{
			Hash:   plumbing.NewHash(v),
			Branch: plumbing.NewBranchReferenceName(k),
			Create: true,
		})
	}

	return r
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

	fmt.Printf("Populated commit %v\n", commit)

	numChildren := len(commit.Children)
	if numChildren == 0 {
		// Reached the last commit, nothing more to traverse
		fmt.Printf("Reached the end commit %s with %v\n", hash, commit)
		return
	}

	for _, chash := range commit.Children {
		cc := Commits[chash]
		if len(cc.Parents) == 1 {
			// The child commit has only one parent, that is this commit.
			// Continue traversing with the parent span and monoclock
			bfs(chash, span, monoclock)
		}
		if len(cc.Parents) > 1 {
			fmt.Printf("Child commit %s is a merge commit with parents %v\n", chash, cc.Parents)
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
				fmt.Printf("Skipping merge commit %c becuase parents %v not traversed\n", chash, parentsNotVisited)
				continue
			}

			// Reached here because all parents have been visited

			// compute spans below...
			// Start with a new span
			ccspan := nextSpan
			nextSpan++
			// Set the child spans for the parents and parent spans for this commit
			for _, ccphash := range cc.Parents {
				Commits[ccphash].ChildSpan = append(Commits[ccphash].ChildSpan, ccspan)
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

			fmt.Printf("Traversing merge commit %s with span %d and reference clock %v\n",
				chash, ccspan, ccmonoclock)
			bfs(chash, ccspan, ccmonoclock)
		}
	}
}

func IterBranchesForCommits(repo *git.Repository) {
	fmt.Printf("%s: IterBranchesForCommits invoked\n", time.Now().String())
	bIter, err := repo.Branches()
	CheckIfError(err)
	// There can be more than one root (initial commit) for a repo.
	roots := make(map[string]string)
	CheckIfError(bIter.ForEach(func(ref *plumbing.Reference) error {
		bName := ref.Name().String()
		bType := ref.Type()
		fmt.Printf("%s: %s\n", time.Now().String(), PrintMemUsage())
		fmt.Printf("%s: Iterating Branch: %s, %s, %s\n", time.Now().String(), bName, bType, ref.Hash().String())

		// Since this is the tip of the branch and git commit does not have a pointer
		// to the children, we set the child commit to empty string.
		rets := dfs(repo, ref.Hash(), plumbing.NewHash(""), bName)
		for _, r := range rets {
			roots[r] = bName
			fmt.Printf("Found root %s traversing branch %s\n", r, bName)
		}
		return nil
	}))

	fmt.Printf("%s: %s\n", time.Now().String(), PrintMemUsage())

	// Set all initial commits
	for c, b := range roots {
		Commits[b].InitialCommit = true
		fmt.Printf("%s: Root of the repo is %s in %s\n", time.Now().String(), c, b)
	}

	// Mark all Tags
	for t, c := range Tags {
		Commits[c].Tags = append(Commits[c].Tags, t)
		fmt.Printf("Tag %s: %s\n", t, c)
	}

	// Mark all branch heads
	for b, c := range Branches {
		Commits[c].BranchHeads = append(Commits[c].Tags, b)
		fmt.Printf("Tag %s: %s\n", b, c)
	}

	// Traverse the commits from the root and set spans and inferred time for each commit
	for hash, _ := range roots {
		// Each root stats with a new span
		nextSpan++

		// Initialize span and moncolock for this root
		span := nextSpan
		monoclock := Commits[hash].CommitWhen
		bfs(hash, span, monoclock)
	}

}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func PrintMemUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	return fmt.Sprintf("Alloc = %v MiB\tTotalAlloc = %v MiB\tSys = %v MiB\tNumGC = %v",
		bToMb(m.Alloc), bToMb(m.TotalAlloc), bToMb(m.Sys), m.NumGC)
}

func main() {
	fmt.Printf("%s: Start processing\n", time.Now().String())
	CheckArgs("<path>")
	path := os.Args[1]

	repo := CheckoutBranches(path)
	IterBranchesForCommits(repo)

	<-make(chan int)
}
