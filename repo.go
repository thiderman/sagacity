package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Repo represents a repository of information yaml files.
type Repo struct {
	Key      string
	Info     map[string]Info
	Control  map[string]Info
	Subrepos map[string]Repo
	root     string
	wg       sync.WaitGroup
}

func (r Repo) String() string {
	return fmt.Sprintf("R: %s (%d articles)", r.Key, len(r.Info))
}

// LoadRepos loads multiple repositories and stores them
func LoadRepos(p string) (repos map[string]Repo) {
	repos = make(map[string]Repo)
	p = getPath(p)
	wg := sync.WaitGroup{}

	files, _ := ioutil.ReadDir(p)
	for _, file := range files {
		fn := filepath.Join(p, file.Name())

		if _, err := os.Stat(filepath.Join(fn, "_repo.yaml")); os.IsNotExist(err) {
			log.Println(fmt.Sprintf("Skipping repo %s: no _repo.yaml found.", file.Name()))
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			repos[asKey(fn)] = NewRepo(fn)
		}()
	}

	wg.Wait()
	return
}

// UpdateRepos will run git pull on the repos
func UpdateRepos(repos map[string]Repo) {
	for key, repo := range repos {
		log.Printf("Updating %s...", key)
		repo.git("pull", "origin", "master")
	}
}

// AddRepo clones a new repository
func AddRepo(root, name, url string) {
	dir := filepath.Join(root, name)
	git("", "clone", url, dir)
	log.Print("Repository added!")
}

// NewRepo loads a repository on a path
func NewRepo(p string) (r Repo) {
	r = Repo{Key: asKey(p), root: getPath(p)}
	r.Info = make(map[string]Info)
	r.Control = make(map[string]Info)
	r.Subrepos = make(map[string]Repo)

	filepath.Walk(r.root, r.walk)
	r.wg.Wait()

	return
}

// ListRepos prints a sorted list of available repostiories.
func ListRepos(repos map[string]Repo) {
	keys := make([]string, 0, len(repos))
	for key := range repos {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		fmt.Println(key)
	}
}

// Keys returns a sorted list of the info keys in the repository
func (r *Repo) Keys() []string {
	keys := make([]string, 0, len(r.Info))
	for _, info := range r.Info {
		keys = append(keys, info.ID)
	}

	sort.Strings(keys)

	return keys
}

// SubrepoKeys returns a sorted list of the subrepo keys in the repository
func (r *Repo) SubrepoKeys() []string {
	keys := make([]string, 0, len(r.Subrepos))
	for _, sub := range r.Subrepos {
		keys = append(keys, sub.Key)
	}

	sort.Strings(keys)

	return keys
}

// Execute determine what to do:
//
// If the last argument in the command line from c.Args() given points to a
// repo, the index of the loop will be printed.
//
// If the last argument is an Info item, it will be executed.
func (r *Repo) Execute(c *cli.Context) {
	var repo Repo
	var info Info
	var ok bool
	repo = *r

	// The first argument is not needed since it was used to determine the
	// location to this very repo.
	args := c.Args()[1:]
	for _, arg := range args {
		// If we can find an info with the key provided, execute that right away!
		if info, ok = repo.Info[arg]; ok {
			info.Execute()
			return
		}

		// Otherwise, check if we have a subrepo matching the argument If we do,
		// `repo will be set to the new one, and the next iteration will check
		// deeper into the tree. If not, we need to break the loop.
		if repo, ok = repo.Subrepos[arg]; !ok {
			break
		}
	}

	// Nothing matched, or the end node is a repo. Print the contents,
	// beginning with a blue listing of subrepos
	blue := color.New(color.FgBlue, color.Bold).SprintfFunc()
	for _, key := range repo.SubrepoKeys() {
		fmt.Println(blue(key))
	}
	for _, key := range repo.Keys() {
		fmt.Println(key)
	}
}

func (r *Repo) walk(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Println("walk error: ", err)
		return err
	}

	// Dotfile, like .git or whatever. Skip.
	if strings.HasPrefix(filepath.Base(path), ".") {
		return filepath.SkipDir
	}

	if info.IsDir() && r.isSubrepo(path) {
		r.wg.Add(1)
		go r.loadSubrepo(path)

		// Return SkipDir since the directory will be parsed by the
		// NewRepo call inside of loadSubrepo()
		return filepath.SkipDir

	} else if strings.HasSuffix(path, ".yaml") {
		r.wg.Add(1)
		go r.loadInfo(path)
	}

	return nil
}

func (r *Repo) loadInfo(path string) {
	defer r.wg.Done()

	info, err := LoadInfo(path)
	if err != nil {
		log.Println("Failed to load info: ", err)
	}

	// Control files start with an underscore and should not be stored as
	// normal Info documents.
	if strings.HasPrefix(asKey(path), "_") {
		r.Control[info.ID] = info
	} else {
		r.Info[info.ID] = info
	}
}

func (r *Repo) loadSubrepo(path string) {
	defer r.wg.Done()
	nr := NewRepo(path)
	r.Subrepos[nr.Key] = nr
}

func (r *Repo) isSubrepo(path string) bool {
	// This is the root...
	if r.root == path {
		return false
	}

	return true
}

// Helper to run git commands inside of a repository
func (r *Repo) git(args ...string) {
	git(r.root, args...)
}
