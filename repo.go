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
	GitRemote   *git.Remote
	GitWorktree *git.Worktree

	/* Objects within a given repo */
	// Map branch name to commit hash
	Branches map[string]string
	// Map tag name to commit hash
	Tags map[string]string
	// Map commit hash to commit object
	Commits map[string]*Commit

	// The next span to be assigned
	NextSpan uint32
}

func (repo Repo) PrintBranches() {
	// Log all the branches
	for b, c := range repo.Branches {
		fmt.Printf("%s: Branch %s at %s\n", repo.Name, b, c)
	}
}

func (repo Repo) PrintTags() {
	// Log all the Tags
	for t, c := range repo.Tags {
		fmt.Printf("%s: Tag %s at %s\n", repo.Name, t, c)
	}
}

func (repo Repo) Open() {
	var err error
	fmt.Printf("%s: %s: Open local repo clone path\n", time.Now().String(), repo.Path)
	// We instantiate a new repository targeting the given path (the .git folder)
	repo.GitRepo, err = git.PlainOpen(repo.Path)
	CheckIfError(err)

	remotes, err := repo.GitRepo.Remotes()
	CheckIfError(err)
	repo.GitRemote = remotes[0]
	fmt.Printf("%s: %s: Remotes for %s\n", time.Now().String(), repo.Path, repo.GitRemote)
	repo.GitRepo.Fetch(&git.FetchOptions{})

	// Get the worktree to check out branches locally
	repo.GitWorktree, err = repo.GitRepo.Worktree()
	CheckIfError(err)
	fmt.Println("%s: Worktree: ", repo.Path, repo.GitWorktree)

	// Set repo name from the Remote Fetch URL
	// TODO: Strip everything other than the repo name later
	repo.Name = repo.GitRemote.Config().URLs[0]
}

func (repo Repo) ListRemoteBranches() {
	// List the remote branches and tags
	fmt.Printf("%s: %s: Listing Remote branches and tags for %s\n", time.Now().String(), repo.Name, remote)
	refList, err := repo.GitRemote.List(&git.ListOptions{})
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

func (repo Repo) CheckoutBranches() {
	// checkout branches locally
	fmt.Printf("%s: %s: Checking out Branches locally \n", time.Now().String(), repo.Name)
	for k, v := range repo.Branches {
		fmt.Printf("%s: %s: %s\n", time.Now().String(), repo.Name, PrintMemUsage())
		fmt.Printf("%s: %s: Checking out... %s at %s\n", time.Now().String(), repo.Name, k, v)
		repo.GitWorktree.Checkout(&git.CheckoutOptions{
			Hash:   plumbing.NewHash(v),
			Branch: plumbing.NewBranchReferenceName(k),
			Create: true,
		})
	}
}

func (repo Repo) PopulateCommits() {
	fmt.Printf("%s: %s: Populate commits by iterating branches\n", time.Now().String(), repo.Name)
	bIter, err := repo.GitRepo.Branches()
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
		Commits[c].InitialCommit = true
		fmt.Printf("%s: Root of the repo is %s in %s\n", time.Now().String(), c, b)
	}

	// Mark all Tags
	for t, hstr := range Tags {
		if commit, ok := Commits[hstr]; ok {
			commit.Tags = append(commit.Tags, t)
			fmt.Printf("%s: Tag %s\n", hstr, t)
		} else {
			fmt.Printf("%s: Not found commit, Tag %s\n", hstr, t)
		}
	}

	// Mark all branch heads
	for b, hstr := range Branches {
		Commits[hstr].BranchHeads = append(Commits[hstr].Tags, b)
		fmt.Printf("%s: Branch %s\n", hstr, b)
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

func main() {
	fmt.Printf("%s: Start processing\n", time.Now().String())
	CheckArgs("<path>")
	path := os.Args[1]

	repo := Repo{Path: path}
	repo.Open()
	repo.ListRemoteBranches()
	repo.CheckoutBranches()
	repo.PopulateCommits()

	//<-make(chan int)
}
