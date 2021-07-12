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

// All data per repo
type Repo struct {
	/* Identifiers of the repo */
	// Local path
	Path string
	// Name extracted from the fetch URL
	Name string

	/* git Repository objects */
	GitRepo     *git.Repository
	GitWorktree *git.Worktree

	/* Objects within a given repo */
	// Map branch name to commit hash
	Branches map[string]string
	// Map tag name to commit hash
	Tags map[string]string
	// Map commit hash to commit object
	Commits map[string]*Commit
	// Map of all initial commits (roots) to the branch discovered from
	Roots map[string]string

	// The next span to be assigned
	nextSpan uint32
}

func (repo Repo) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(fmt.Sprintf("%s: %s", repo.Name, format), a...)
}

func (repo Repo) PrintBranches() {
	// Log all the branches
	for b, c := range repo.Branches {
		repo.Printf("%s: Branch %s at %s\n", b, c)
	}
}

func (repo Repo) PrintTags() {
	// Log all the Tags
	for t, c := range repo.Tags {
		repo.Printf("%s: Tag %s at %s\n", t, c)
	}
}

func (repo *Repo) Open() {
	var err error
	fmt.Printf("%s: %s: Open local repo clone path\n", time.Now().String(), repo.Path)
	// We instantiate a new repository targeting the given path (the .git folder)
	repo.GitRepo, err = git.PlainOpen(repo.Path)
	CheckIfError(err)

	remotes, err := repo.GitRepo.Remotes()
	CheckIfError(err)
	remote := remotes[0]
	fmt.Printf("%s: %s: Remotes for %s\n", time.Now().String(), repo.Path, remote)
	repo.GitRepo.Fetch(&git.FetchOptions{})

	// Get the worktree to check out branches locally
	repo.GitWorktree, err = repo.GitRepo.Worktree()
	CheckIfError(err)
	fmt.Printf("%s: %s: Worktree: %v\n", time.Now().String(), repo.Path, repo.GitWorktree)

	// Set repo name from the Remote Fetch URL
	// TODO: Strip everything other than the repo name later
	repo.Name = strings.Split(strings.Split(remote.Config().URLs[0], "/")[1], ".")[0]

	repo.Branches = make(map[string]string)
	repo.Tags = make(map[string]string)
	repo.Roots = make(map[string]string)
	repo.Commits = make(map[string]*Commit)
	repo.nextSpan = SPAN_NOT_SET

	// List the remote branches and tags
	fmt.Printf("%s: Listing Remote branches and tags for %v\n", time.Now().String(), remote)
	refList, err := remote.List(&git.ListOptions{})
	CheckIfError(err)
	for _, ref := range refList {
		refName := ref.Name().String()
		hash := ref.Hash().String()
		if strings.HasPrefix(refName, REF_BRANCH_PREFIX) {
			branchName := refName[len(REF_BRANCH_PREFIX):]
			repo.Branches[branchName] = hash
		} else {
			if strings.HasPrefix(refName, REF_TAG_PREFIX) {
				tagName := refName[len(REF_TAG_PREFIX):]
				repo.Tags[tagName] = hash
			}
		}
	}
}

func (repo *Repo) CheckoutBranches() {
	// checkout branches locally
	repo.Printf("%s: Checking out Branches locally \n", time.Now().String())
	for k, v := range repo.Branches {
		repo.Printf("%s: %s\n", time.Now().String(), PrintMemUsage())
		repo.Printf("%s: Checking out... %s at %s\n", time.Now().String(), k, v)
		repo.GitWorktree.Checkout(&git.CheckoutOptions{
			Hash:   plumbing.NewHash(v),
			Branch: plumbing.NewBranchReferenceName(k),
			Create: true,
		})
	}
}

// Return the first commit, the root of the tree
func (repo *Repo) dfsCommitsFromTip(bName string, hash plumbing.Hash, chash plumbing.Hash) []string {
	hstr := hash.String()
	cstr := chash.String()

	// If commit hash is already encountered, then we are reaching the commit from a different
	// span. So, add the child hash to the commit and stop the traversal.
	if commit, ok := repo.Commits[hstr]; ok {
		if cstr != NIL_COMMIT {
			// Append the child hash only if it is known
			repo.Commits[hstr].Children = append(repo.Commits[hstr].Children, cstr)
		}
		if len(repo.Commits[hstr].Children) > 1 {
			repo.Printf("Commit already exists (split): %s <- %s, %v\n", hash, cstr, commit)
		} else {
			repo.Printf("Commit already exists: %s <- %s, %v\n", hash, cstr, commit)
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
	c, err := repo.GitRepo.CommitObject(hash)
	CheckIfError(err)

	repo.Commits[hstr] = &Commit{
		Hash:       hstr,
		CommitWhen: c.Committer.When,
		AuthorWhen: c.Author.When,
		Children:   make([]string, 0),
		Parents:    make([]string, 0),
		Tags:       make([]string, 0),
		Branches:   make(map[string]int32),
	}
	// Update the branch. Need to revisit the entire branch logic later
	repo.Commits[hstr].Branches[bName] = 1
	// Store the child hash ONLY if it is not ""
	if cstr != NIL_COMMIT {
		repo.Commits[hstr].Children = append(repo.Commits[hstr].Children, cstr)
	}

	repo.Printf("Storing commit: %s <- %s, %v\n", hstr, cstr, repo.Commits[hstr])

	// Update the parents
	root := []string{}
	if len(c.ParentHashes) == 0 {
		// Reached the first commit (root), nothing more to traverse
		repo.Printf("Reached the first commit %s\n", hstr)
		root = append(root, hstr)
	} else {
		numParents := len(c.ParentHashes)
		if numParents > 1 {
			repo.Printf("Merge commit %s from %d commits\n", hstr, numParents)
		}
		for _, phash := range c.ParentHashes {
			repo.Commits[hstr].Parents = append(repo.Commits[hstr].Parents, phash.String())
			rets := repo.dfsCommitsFromTip(bName, phash, hash)
			root = append(root, rets...)
		}
	}

	return root
}

func (repo *Repo) ReadCommits() {
	repo.Printf("%s: Populate commits by iterating branches\n", time.Now().String())
	bIter, err := repo.GitRepo.Branches()
	CheckIfError(err)

	// There can be more than one root (initial commit) for a repo.
	CheckIfError(bIter.ForEach(func(ref *plumbing.Reference) error {
		bName := ref.Name().String()
		repo.Printf("%s: %s\n", time.Now().String(), PrintMemUsage())
		repo.Printf("%s: %s: Iterating Branch from hash %s\n", time.Now().String(), bName, ref.Hash().String())

		// Since this is the tip of the branch and git commit does not have a pointer
		// to the children, we set the child commit to empty string.
		rets := repo.dfsCommitsFromTip(bName, ref.Hash(), plumbing.NewHash(""))
		for _, r := range rets {
			repo.Roots[r] = bName
			repo.Printf("%s: Found root %s traversing branch\n", r)
		}
		return nil
	}))

	// Set all initial commits
	for c, b := range repo.Roots {
		repo.Commits[c].InitialCommit = true
		repo.Printf("%s: Root of the repo is %s in %s\n", time.Now().String(), c, b)
	}

	// Mark all Tags
	for t, hstr := range repo.Tags {
		if commit, ok := repo.Commits[hstr]; ok {
			commit.Tags = append(commit.Tags, t)
			repo.Printf("%s: Tag %s\n", hstr, t)
		} else {
			repo.Printf("%s: Not found commit, Tag %s\n", hstr, t)
		}
	}

	// Mark all branch heads
	for b, hstr := range repo.Branches {
		repo.Commits[hstr].BranchHeads = append(repo.Commits[hstr].Tags, b)
		repo.Printf("%s: Branch %s\n", hstr, b)
	}
}

func (repo *Repo) dfsCommitsFromRoot(hash string, span uint32, monoclock time.Time) {
	commit := repo.Commits[hash]
	commit.Span = span
	if commit.CommitWhen.Before(monoclock) {
		// set the InferredTime to mono clock + 1 millisecond
		monoclock.Add(time.Millisecond)
		commit.InferredTime = monoclock
	} else {
		commit.InferredTime = commit.CommitWhen
		monoclock = commit.InferredTime
	}

	repo.Printf("%s: Populated commit %v\n", hash, commit)

	numChildren := len(commit.Children)
	if numChildren == 0 {
		// Reached the last commit, nothing more to traverse
		repo.Printf("%s: Reached the end commit with %v\n", hash, commit)
		return
	}

	for _, chash := range commit.Children {
		cc := repo.Commits[chash]
		// skip the child commit if it has already by visited from a prior root
		if cc.Span != SPAN_NOT_SET {
			repo.Printf("%s: Child commit is already visited, skipping %v\n", chash, cc)
			continue
		}

		if len(cc.Parents) == 1 {
			// If this commit is a split commit, i.e has more than one child commit, then
			// increment the span for the child commit
			ccspan := span
			if numChildren > 1 {
				repo.nextSpan++
				ccspan = repo.nextSpan
				commit.ChildSpan = append(commit.ChildSpan, ccspan)
				repo.Printf("%s: Updated commit %v\n", hash, commit)
				cc.ParentSpan = append(cc.ParentSpan, span)
			}
			// Continue traversing the child commit as it has only one parent
			repo.dfsCommitsFromRoot(chash, ccspan, monoclock)
		}
		if len(cc.Parents) > 1 {
			fmt.Printf("%s: Child commit is a merge commit with parents %v\n", chash, cc.Parents)
			// The child happens to be a merge commit, the child will have multiple parent commits
			// Traverse only those child commit(s) that have all their parents visited
			// Visited can be verified by looking into the span or the inferred time.
			parentsNotVisited := []string{}
			for _, phash := range cc.Parents {
				if repo.Commits[phash].Span == SPAN_NOT_SET {
					parentsNotVisited = append(parentsNotVisited, phash)
				}
			}
			if len(parentsNotVisited) > 0 {
				repo.Printf("%s: Skipping merge commit becuase parents %v not traversed\n", chash, parentsNotVisited)
				continue
			}

			// Reached here because all parents have been visited

			// compute spans below...
			// Start with a new span
			repo.nextSpan++
			ccspan := repo.nextSpan
			// Set the child spans for the parents and parent spans for this commit
			for _, ccphash := range cc.Parents {
				repo.Commits[ccphash].ChildSpan = append(repo.Commits[ccphash].ChildSpan, ccspan)
				repo.Printf("%s: Updated commit %v\n", ccphash, repo.Commits[ccphash])
				cc.ParentSpan = append(cc.ParentSpan, repo.Commits[ccphash].Span)
			}

			// compute monoclock below...
			// The monoclock for the child has to be after the monoclock of all the parents
			ccmonoclock := monoclock
			for _, ccphash := range cc.Parents {
				if ccmonoclock.Before(repo.Commits[ccphash].InferredTime) {
					ccmonoclock = repo.Commits[ccphash].InferredTime
				}
			}

			repo.Printf("%s: Traversing merge commit with span %d and reference clock %v\n", chash, ccspan, ccmonoclock)
			repo.dfsCommitsFromRoot(chash, ccspan, ccmonoclock)
		}
	}
}

func (repo *Repo) PopulateExtraCommitFields() {
	// Traverse the commits from the root and set spans and inferred time for each commit
	for hash, _ := range repo.Roots {
		// Each root stats with a new span
		repo.nextSpan++

		// Initialize span and moncolock for this root
		span := repo.nextSpan
		monoclock := repo.Commits[hash].CommitWhen
		repo.dfsCommitsFromRoot(hash, span, monoclock)
	}
}

func main() {
	fmt.Printf("%s: Start processing\n", time.Now().String())
	CheckArgs("<path>")
	path := os.Args[1]

	repo := Repo{Path: path}
	repo.Open()
	repo.Printf("%s: Done Initializing %s\n", time.Now().String(), PrintMemUsage())

	repo.CheckoutBranches()
	repo.Printf("%s: Done Checking out branches %s\n", time.Now().String(), PrintMemUsage())

	repo.ReadCommits()
	repo.Printf("%s: Done Reading commits %s\n", time.Now().String(), PrintMemUsage())

	repo.PopulateExtraCommitFields()
	repo.Printf("%s: Done populating extra commit fields %s\n", time.Now().String(), PrintMemUsage())
	//<-make(chan int)
}
