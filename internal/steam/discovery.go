package steam

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// Installation identifies a Steam application and its installed directory.
type Installation struct {
	AppID       string
	Name        string
	InstallPath string
}

// Discover enumerates manifests in the supplied Steam roots and their configured libraries.
func Discover(roots []string) ([]Installation, error) {
	libraries := make([]string, 0)
	seenLibraries := make(map[string]struct{})
	for _, root := range roots {
		root, err := existingDirectory(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		addLibrary(&libraries, seenLibraries, root)
		libraryFile := filepath.Join(root, "steamapps", "libraryfolders.vdf")
		data, err := os.ReadFile(libraryFile)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("steam: cannot read %s: %v", libraryFile, err)
			}
			continue
		}
		value, err := Parse(data)
		if err != nil {
			log.Printf("steam: cannot parse %s: %v", libraryFile, err)
			continue
		}
		folders, ok := value.Object("libraryfolders")
		if !ok {
			continue
		}
		for key, folder := range folders.object {
			if _, err := strconv.ParseUint(key, 10, 64); err != nil {
				continue
			}
			path, ok := folder.String("path")
			if !ok || path == "" {
				continue
			}
			if _, err := existingDirectory(path); err != nil {
				if !os.IsNotExist(err) {
					log.Printf("steam: cannot inspect library %s: %v", path, err)
				}
				continue
			}
			addLibrary(&libraries, seenLibraries, path)
		}
	}

	installations := make([]Installation, 0)
	for _, library := range libraries {
		patterns, err := filepath.Glob(filepath.Join(library, "steamapps", "appmanifest_*.acf"))
		if err != nil {
			return nil, fmt.Errorf("glob Steam manifests in %q: %w", library, err)
		}
		sort.Strings(patterns)
		for _, manifestPath := range patterns {
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				log.Printf("steam: cannot read %s: %v", manifestPath, err)
				continue
			}
			manifest, err := ParseManifest(data)
			if err != nil {
				log.Printf("steam: cannot parse %s: %v", manifestPath, err)
				continue
			}
			installations = append(installations, Installation{
				AppID: manifest.AppID, Name: manifest.Name,
				InstallPath: filepath.Join(library, "steamapps", "common", manifest.InstallDir),
			})
		}
	}
	sort.SliceStable(installations, func(i, j int) bool {
		if installations[i].AppID != installations[j].AppID {
			return installations[i].AppID < installations[j].AppID
		}
		return installations[i].InstallPath < installations[j].InstallPath
	})
	return installations, nil
}

func existingDirectory(path string) (string, error) {
	clean := filepath.Clean(path)
	info, err := os.Stat(clean)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", clean)
	}
	return clean, nil
}

func addLibrary(libraries *[]string, seen map[string]struct{}, path string) {
	path = filepath.Clean(path)
	if _, ok := seen[path]; ok {
		return
	}
	seen[path] = struct{}{}
	*libraries = append(*libraries, path)
}
