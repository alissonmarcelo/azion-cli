package init

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	msg "github.com/aziontech/azion-cli/messages/edge_applications"
	"github.com/aziontech/azion-cli/pkg/cmdutil"
	"github.com/aziontech/azion-cli/pkg/contracts"
	"github.com/aziontech/azion-cli/pkg/iostreams"
	"github.com/aziontech/azion-cli/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type InitInfo struct {
	Name           string
	TypeLang       string
	PathWorkingDir string
	YesOption      bool
	NoOption       bool
}

var (
	TemplateBranch = "main"
	TemplateMajor  = "0"
)

const (
	REPO string = "https://github.com/aziontech/azioncli-template.git"
)

type InitCmd struct {
	Io            *iostreams.IOStreams
	GetWorkDir    func() (string, error)
	FileReader    func(path string) ([]byte, error)
	CommandRunner func(cmd string, envvars []string) (string, int, error)
	LookPath      func(bin string) (string, error)
	IsDirEmpty    func(dirpath string) (bool, error)
	CleanDir      func(dirpath string) error
	WriteFile     func(filename string, data []byte, perm fs.FileMode) error
	OpenFile      func(name string) (*os.File, error)
	RemoveAll     func(path string) error
	Rename        func(oldpath string, newpath string) error
	CreateTempDir func(dir string, pattern string) (string, error)
	EnvLoader     func(path string) ([]string, error)
	Stat          func(path string) (fs.FileInfo, error)
	Mkdir         func(path string, perm os.FileMode) error
	GitPlainClone func(path string, isBare bool, o *git.CloneOptions) (*git.Repository, error)
}

func NewInitCmd(f *cmdutil.Factory) *InitCmd {
	return &InitCmd{
		Io:         f.IOStreams,
		GetWorkDir: utils.GetWorkingDir,
		FileReader: os.ReadFile,
		CommandRunner: func(cmd string, envvars []string) (string, int, error) {
			return utils.RunCommandWithOutput(envvars, cmd)
		},
		LookPath:      exec.LookPath,
		IsDirEmpty:    utils.IsDirEmpty,
		CleanDir:      utils.CleanDirectory,
		WriteFile:     os.WriteFile,
		OpenFile:      os.Open,
		RemoveAll:     os.RemoveAll,
		Rename:        os.Rename,
		CreateTempDir: os.MkdirTemp,
		EnvLoader:     utils.LoadEnvVarsFromFile,
		Stat:          os.Stat,
		Mkdir:         os.MkdirAll,
		GitPlainClone: git.PlainClone,
	}
}

func NewCobraCmd(init *InitCmd) *cobra.Command {
	options := &contracts.AzionApplicationOptions{}
	info := &InitInfo{}
	cobraCmd := &cobra.Command{
		Use:           msg.EdgeApplicationsInitUsage,
		Short:         msg.EdgeApplicationsInitShortDescription,
		Long:          msg.EdgeApplicationsInitLongDescription,
		SilenceUsage:  true,
		SilenceErrors: true,
		Example: heredoc.Doc(`
		$ azioncli edge_applications init
		$ azioncli edge_applications init --help
		$ azioncli edge_applications init --name "thisisatest" --type javascript
		$ azioncli edge_applications init --name "thisisatest" --type flareact
		$ azioncli edge_applications init --name "thisisatest" --type nextjs
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return init.run(info, options, cmd)
		},
	}
	cobraCmd.Flags().StringVar(&info.Name, "name", "", msg.EdgeApplicationsInitFlagName)
	cobraCmd.Flags().StringVar(&info.TypeLang, "type", "", msg.EdgeApplicationsInitFlagType)
	cobraCmd.Flags().BoolVarP(&info.YesOption, "yes", "y", false, msg.EdgeApplicationsInitFlagYes)
	cobraCmd.Flags().BoolVarP(&info.NoOption, "no", "n", false, msg.EdgeApplicationsInitFlagNo)

	return cobraCmd
}

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	return NewCobraCmd(NewInitCmd(f))
}

func (cmd *InitCmd) run(info *InitInfo, options *contracts.AzionApplicationOptions, c *cobra.Command) error {
	if info.YesOption && info.NoOption {
		return msg.ErrorYesAndNoOptions
	}

	path, err := cmd.GetWorkDir()
	if err != nil {
		return err
	}

	bytePackageJson, pathPackageJson, err := ReadPackageJson(cmd, path)
	if err != nil {
		return err
	}

	projectName, projectSettings, err := DetectedProjectJS(bytePackageJson)
	if err != nil {
		return err
	}

	if info.TypeLang == "cdn" {
		err = updateProjectName(c, info, cmd, projectName, bytePackageJson, path)
		if err != nil {
			return err
		}
		return initCdn(cmd, path, info)
	}

	fmt.Fprintf(cmd.Io.Out, msg.EdgeApplicationsAutoDetectec, projectSettings) // nolint:all

	if !hasThisFlag(c, "type") {
		info.TypeLang = projectSettings
		fmt.Fprintf(cmd.Io.Out, "%s\n", msg.EdgeApplicationsInitTypeNotSent) // nolint:all
	}

	//gets the test function (if it could not find it, it means it is currently not supported)
	testFunc, ok := makeTestFuncMap(cmd.Stat)[info.TypeLang]
	if !ok {
		return utils.ErrorUnsupportedType
	}

	info.PathWorkingDir = path

	options.Test = testFunc
	if err = options.Test(info.PathWorkingDir); err != nil {
		return err
	}

	shouldFetchTemplates, err := shouldFetch(cmd, info)
	if err != nil {
		return err
	}

	if shouldFetchTemplates {
		if err = cmd.fetchTemplates(info); err != nil {
			return err
		}

		if err = UpdateScript(cmd, bytePackageJson, pathPackageJson); err != nil {
			return err
		}

		err = updateProjectName(c, info, cmd, projectName, bytePackageJson, info.PathWorkingDir)
		if err != nil {
			return err
		}

		if err = cmd.organizeJsonFile(options, info); err != nil {
			return err
		}

		fmt.Fprintf(cmd.Io.Out, "%s\n", msg.WebAppInitCmdSuccess)                                // nolint:all
		fmt.Fprintf(cmd.Io.Out, fmt.Sprintf(msg.EdgeApplicationsInitSuccessful+"\n", info.Name)) // nolint:all
	}

	err = cmd.runInitCmdLine(info)
	if err != nil {
		return err
	}

	return nil
}

func hasThisFlag(c *cobra.Command, flag string) bool {
	return c.Flags().Changed(flag)
}

type PackageJson struct {
	Name         string `json:"name"`
	Dependencies struct {
		Next     string `json:"next"`
		Flareact string `json:"flareact"`
	} `json:"dependencies"`
}

func ReadPackageJson(cmd *InitCmd, path string) ([]byte, string, error) {
	pathPackageJson := path + "/package.json"
	bytePackageJson, err := cmd.FileReader(pathPackageJson)
	if err != nil {
		return []byte(""), "", msg.ErrorPackageJsonNotFound
	}
	return bytePackageJson, pathPackageJson, nil
}

func DetectedProjectJS(bytePackageJson []byte) (projectName string, projectSettings string, err error) {
	var packageJson PackageJson
	err = json.Unmarshal(bytePackageJson, &packageJson)
	if err != nil {
		return "", "", utils.ErrorUnmarshalReader
	}

	if len(packageJson.Dependencies.Next) > 0 {
		projectName = packageJson.Name
		projectSettings = "nextjs"
	} else if len(packageJson.Dependencies.Flareact) > 0 {
		projectName = packageJson.Name
		projectSettings = "flareact"
	} else {
		projectName = packageJson.Name
		projectSettings = "javascript"
	}

	return projectName, projectSettings, nil
}

func (cmd *InitCmd) fetchTemplates(info *InitInfo) error {
	//create temporary directory to clone template into
	dir, err := cmd.CreateTempDir(info.PathWorkingDir, ".template")
	if err != nil {
		return utils.ErrorInternalServerError
	}
	defer func() {
		_ = cmd.RemoveAll(dir)
	}()

	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{URL: REPO})
	if err != nil {
		return utils.ErrorFetchingTemplates
	}

	tags, err := r.Tags()
	if err != nil {
		return msg.ErrorGetAllTags
	}

	tag, err := sortTag(tags, TemplateMajor, TemplateBranch)
	if err != nil {
		return msg.ErrorIterateAllTags
	}

	_, err = cmd.GitPlainClone(dir, false, &git.CloneOptions{
		URL:           REPO,
		ReferenceName: plumbing.ReferenceName(tag),
	})
	if err != nil {
		return utils.ErrorFetchingTemplates
	}

	azionDir := info.PathWorkingDir + "/azion"

	//move contents from temporary directory into final destination
	err = cmd.Rename(dir+"/webdev/"+info.TypeLang, azionDir)
	if err != nil {
		return utils.ErrorMovingFiles
	}

	return nil
}

func sortTag(tags storer.ReferenceIter, major, branch string) (string, error) {
	var tagCurrent int = 0
	var tagCurrentStr string
	var tagWithMajorOk int = 0
	var tagWithMajorOKStr string
	var err error
	err = tags.ForEach(func(t *plumbing.Reference) error {
		tagFormat := formatTag(string(t.Name()))
		tagFormat = checkBranch(tagFormat, branch)
		if tagFormat != "" {
			if strings.Split(tagFormat, "")[0] == major {
				var numberTag int
				numberTag, err = strconv.Atoi(tagFormat)
				if numberTag > tagWithMajorOk {
					tagWithMajorOk = numberTag
					tagWithMajorOKStr = string(t.Name())
				}
			} else {
				var numberTag int
				numberTag, err = strconv.Atoi(tagFormat)
				if numberTag > tagCurrent {
					tagCurrent = numberTag
					tagCurrentStr = string(t.Name())
				}
			}
		}
		return err
	})

	if tagWithMajorOKStr != "" {
		return tagWithMajorOKStr, err
	}

	return tagCurrentStr, err
}

// formatTag slice tag by '/' taking index 2 where the version is, transforming it into a list taking only the numbers
func formatTag(tag string) string {
	var t string
	for _, v := range strings.Split(strings.Split(tag, "/")[2], "") {
		if _, err := strconv.Atoi(v); err == nil {
			t += v
		}
	}
	return t
}

func checkBranch(num, branch string) string {
	if branch == "dev" {
		if len(num) == 4 {
			return num
		}
	} else if len(num) == 3 {
		return num
	}
	return ""
}

func (cmd *InitCmd) runInitCmdLine(info *InitInfo) error {
	var err error

	_, err = cmd.LookPath("npm")
	if err != nil {
		return msg.ErrorNpmNotInstalled
	}

	conf, err := getConfig(cmd, info.PathWorkingDir)
	if err != nil {
		return err
	}

	envs, err := cmd.EnvLoader(conf.InitData.Env)
	if err != nil {
		return msg.ErrReadEnvFile
	}

	switch info.TypeLang {
	case "javascript":
		err = InitJavascript(info, cmd, conf, envs)
		if err != nil {
			return err
		}
	case "nextjs":
		err = InitNextjs(info, cmd, conf, envs)
		if err != nil {
			return err
		}
	case "flareact":
		err = InitFlareact(info, cmd, conf, envs)
		if err != nil {
			return err
		}
	default:
		return utils.ErrorUnsupportedType
	}

	return nil
}

func (cmd *InitCmd) organizeJsonFile(options *contracts.AzionApplicationOptions, info *InitInfo) error {
	file, err := cmd.FileReader(info.PathWorkingDir + "/azion/azion.json")
	if err != nil {
		return msg.ErrorOpeningAzionFile
	}
	err = json.Unmarshal(file, &options)
	if err != nil {
		return msg.ErrorUnmarshalAzionFile
	}
	options.Name = info.Name
	options.Type = info.TypeLang

	data, err := json.MarshalIndent(options, "", "  ")
	if err != nil {
		return msg.ErrorUnmarshalAzionFile
	}

	err = cmd.WriteFile(info.PathWorkingDir+"/azion/azion.json", data, 0644)
	if err != nil {
		return utils.ErrorInternalServerError
	}
	return nil
}

func yesNoFlagToResponse(info *InitInfo) bool {
	if info.YesOption {
		return info.YesOption
	}

	return false
}

func InitJavascript(info *InitInfo, cmd *InitCmd, conf *contracts.AzionApplicationConfig, envs []string) error {
	pathWorker := info.PathWorkingDir + "/worker"
	if err := cmd.Mkdir(pathWorker, os.ModePerm); err != nil {
		return msg.ErrorFailedCreatingWorkerDirectory
	}

	err := runCommand(cmd, conf, envs)
	if err != nil {
		return err
	}

	showInstructions(cmd, `	[ General Instructions ]
	[ Usage ]
		- Build Command: npm run build
		- Publish Command: npm run deploy
	[ Notes ]
		- Node 16x or higher`) //nolint:all

	return nil
}

func InitNextjs(info *InitInfo, cmd *InitCmd, conf *contracts.AzionApplicationConfig, envs []string) error {
	pathWorker := info.PathWorkingDir + "/worker"
	if err := cmd.Mkdir(pathWorker, os.ModePerm); err != nil {
		return msg.ErrorFailedCreatingWorkerDirectory
	}

	err := runCommand(cmd, conf, envs)
	if err != nil {
		return err
	}

	showInstructions(cmd, `	[ General Instructions ]
    - Requirements:
        - Tools: npm
        - AWS Credentials (./azion/webdev.env): AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
        - Customize the path to static content - AWS S3 storage (.azion/kv.json)
    [ Usage ]
    	- Build Command: npm run build
    	- Publish Command: npm run deploy
    [ Notes ]
        - Node 16x or higher`) //nolint:all
	return nil
}

func InitFlareact(info *InitInfo, cmd *InitCmd, conf *contracts.AzionApplicationConfig, envs []string) error {
	pathWorker := info.PathWorkingDir + "/worker"
	if err := cmd.Mkdir(pathWorker, os.ModePerm); err != nil {
		return msg.ErrorFailedCreatingWorkerDirectory
	}

	err := runCommand(cmd, conf, envs)
	if err != nil {
		return err
	}

	if err = cmd.Mkdir(info.PathWorkingDir+"/public", os.ModePerm); err != nil {
		return msg.ErrorFailedCreatingPublicDirectory
	}

	showInstructions(cmd, `	[ General Instructions ]
	- Requirements:
		- Tools: npm
		- AWS Credentials (./azion/webdev.env): AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
		- Customize the path to static content - AWS S3 storage (.azion/kv.json)
	[ Usage ]
		- Install Command: npm install
		- Build Command: npm run build
		- Publish Command: npm run deploy
	[ Notes ]
		- Node 16x or higher`) //nolint:all

	return nil
}

func showInstructions(cmd *InitCmd, instructions string) {
	fmt.Fprintf(cmd.Io.Out, instructions) //nolint:all
}

func getConfig(cmd *InitCmd, path string) (conf *contracts.AzionApplicationConfig, err error) {
	jsonConf := path + "/azion/config.json"
	file, err := cmd.FileReader(jsonConf)
	if err != nil {
		return conf, msg.ErrorOpeningConfigFile
	}
	conf = &contracts.AzionApplicationConfig{}
	err = json.Unmarshal(file, &conf)
	if err != nil {
		return conf, msg.ErrorUnmarshalConfigFile
	}

	return conf, nil
}

func UpdateScript(cmd *InitCmd, packageJson []byte, path string) error {
	packJsonReplaceBuild, err := sjson.Set(string(packageJson), "scripts.build", "azioncli edge_applications build")
	if err != nil {
		return msg.FailedUpdatingScriptsBuildField
	}

	packJsonReplaceDeploy, err := sjson.Set(packJsonReplaceBuild, "scripts.deploy", "azioncli edge_applications publish")
	if err != nil {
		return msg.FailedUpdatingScriptsDeployField
	}

	err = cmd.WriteFile(path, []byte(packJsonReplaceDeploy), 0644)
	if err != nil {
		return fmt.Errorf(utils.ErrorCreateFile.Error(), path)
	}

	return nil
}

func runCommand(cmd *InitCmd, conf *contracts.AzionApplicationConfig, envs []string) error {
	var command string = conf.InitData.Cmd
	if len(conf.InitData.Cmd) > 0 && len(conf.InitData.Default) > 0 {
		command += " && "
	}
	command += conf.InitData.Default

	//if no cmd is specified, we just return nil (no error)
	if command == "" {
		return nil
	}

	switch conf.InitData.OutputCtrl {
	case "disable":
		fmt.Fprintf(cmd.Io.Out, msg.EdgeApplicationsInitRunningCmd)
		fmt.Fprintf(cmd.Io.Out, "$ %s\n", command)

		output, _, err := cmd.CommandRunner(command, envs)
		if err != nil {
			fmt.Fprintf(cmd.Io.Out, "%s\n", output)
			return msg.ErrFailedToRunInitCommand
		}

		fmt.Fprintf(cmd.Io.Out, "%s\n", output)

	case "on-error":
		output, exitCode, err := cmd.CommandRunner(command, envs)
		if exitCode != 0 {
			fmt.Fprintf(cmd.Io.Out, "%s\n", output)
			return msg.ErrFailedToRunInitCommand
		}
		if err != nil {
			return err
		}

	default:
		return msg.EdgeApplicationsOutputErr
	}

	return nil
}

func initCdn(cmd *InitCmd, path string, info *InitInfo) error {
	var err error
	var shouldFetchTemplates bool
	options := &contracts.AzionApplicationCdn{}

	if info.Name == "" {
		jsonConf := path + "/package.json"
		file, err := cmd.FileReader(jsonConf)
		if err != nil {
			return msg.ErrorOpeningAzionFile
		}

		name := gjson.Get(string(file), "type")
		options.Name = name.String()
	}

	shouldFetchTemplates, err = shouldFetch(cmd, info)
	if err != nil {
		return err
	}

	if shouldFetchTemplates {
		pathWorker := path + "/azion"
		if err := cmd.Mkdir(pathWorker, os.ModePerm); err != nil {
			fmt.Println(err)
			return msg.ErrorFailedCreatingAzionDirectory
		}

		options.Name = info.Name
		options.Type = info.TypeLang
		options.Domain.Name = "__DEFAULT__"
		options.Application.Name = "__DEFAULT__"

		data, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return msg.ErrorUnmarshalAzionFile
		}

		err = cmd.WriteFile(path+"/azion/azion.json", data, 0644)
		if err != nil {
			fmt.Println(err)
			return utils.ErrorInternalServerError
		}

		fmt.Fprintf(cmd.Io.Out, fmt.Sprintf(msg.EdgeApplicationsInitSuccessful+"\n", info.Name)) // nolint:all

	}

	return nil
}

func shouldFetch(cmd *InitCmd, info *InitInfo) (bool, error) {
	var response string
	var err error
	var shouldFetchTemplates bool
	if empty, _ := cmd.IsDirEmpty("./azion"); !empty {
		if info.NoOption || info.YesOption {
			shouldFetchTemplates = yesNoFlagToResponse(info)
		} else {
			fmt.Fprintf(cmd.Io.Out, "%s: ", msg.WebAppInitContentOverridden)
			fmt.Fscanln(cmd.Io.In, &response)
			shouldFetchTemplates, err = utils.ResponseToBool(response)
			if err != nil {
				return false, err
			}
		}

		if shouldFetchTemplates {
			err = cmd.CleanDir("./azion")
			if err != nil {
				return false, err
			}
		}
		return shouldFetchTemplates, nil
	}
	return true, nil
}

func updateProjectName(c *cobra.Command, info *InitInfo, cmd *InitCmd, projectName string, bytePackageJson []byte, path string) error {
	//name was not sent through the --name flag
	if !hasThisFlag(c, "name") {
		info.Name = projectName
		fmt.Fprintf(cmd.Io.Out, "%s\n", msg.EdgeApplicationsInitNameNotSent)
	} else {
		updatePackageJson, err := sjson.Set(string(bytePackageJson), "name", info.Name)
		if err != nil {
			return msg.FailedUpdatingNameField
		}
		fmt.Fprintf(cmd.Io.Out, "%s\n", msg.EdgeApplicationsUpdateNamePackageJson)
		path = path + "/package.json"

		err = cmd.WriteFile(path, []byte(updatePackageJson), 0644)
		if err != nil {
			return fmt.Errorf(utils.ErrorCreateFile.Error(), path)
		}
	}

	return nil
}