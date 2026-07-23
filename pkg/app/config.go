package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FangcunMount/component-base/pkg/util/homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// configFlagName 配置标志名称
const configFlagName = "config"

// cfgFile 配置文件
var cfgFile string

// init 初始化配置标志
func init() {
	pflag.StringVarP(&cfgFile, "config", "c", cfgFile, "Read configuration from specified `FILE`, "+
		"support JSON, TOML, YAML, HCL, or Java properties formats.")
}

// addConfigFlag 添加配置标志
func addConfigFlag(basename string, fs *pflag.FlagSet) {
	// 添加配置标志
	fs.AddFlag(pflag.Lookup(configFlagName))

	// 自动设置环境变量
	viper.AutomaticEnv()
	// 设置环境变量前缀
	viper.SetEnvPrefix(strings.Replace(strings.ToUpper(basename), "-", "_", -1))
	// 设置环境变量键替换
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// 初始化配置
	cobra.OnInitialize(func() {
		// 若指定了配置文件地址，则直接使用
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
			fmt.Printf("[config] SetConfigFile -> %s\n", cfgFile)
		} else {
			// 没有指定配置文件地址，则使用当前目录
			viper.AddConfigPath(".")
			fmt.Println("[config] AddConfigPath -> .")
			// 添加 configs 目录
			viper.AddConfigPath("configs")
			fmt.Println("[config] AddConfigPath -> configs")

			// 如果basename包含多个单词，则添加配置路径
			if names := strings.Split(basename, "-"); len(names) > 1 {
				// 添加用户家目录下的配置路径
				userPath := filepath.Join(homedir.HomeDir(), "."+names[0])
				viper.AddConfigPath(userPath)
				fmt.Printf("[config] AddConfigPath -> %s\n", userPath)
				// 添加系统配置路径
				systemPath := filepath.Join("/etc", names[0])
				viper.AddConfigPath(systemPath)
				fmt.Printf("[config] AddConfigPath -> %s\n", systemPath)
			}

			// 设置配置文件名
			viper.SetConfigName(basename)
			fmt.Printf("[config] SetConfigName -> %s\n", basename)
		}

		// 读取配置文件
		if err := viper.ReadInConfig(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: failed to read configuration file(%s): %v\n", cfgFile, err)
			os.Exit(1)
		}

		// 打印配置信息
		fmt.Printf("[config] ConfigFileUsed -> %s\n", viper.ConfigFileUsed())
	})
}
