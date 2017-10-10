package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/euforic/graphql-gen-go/generator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	pkgName string
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "graphql-gen-go",
	Short: "",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fileData := &bytes.Buffer{}

		files := args

		for _, file := range files {
			f, err := ioutil.ReadFile(file)
			if err != nil {
				log.Fatal(err)
			}
			fileData.WriteString("\n")
			fileData.Write(f)
		}
		g := generator.New()
		err := g.Parse(fileData.Bytes())
		if err != nil {
			log.Fatal(err)
		}
		_, typs := g.SetPkgName(pkgName).GenSchemaResolversFile()
		for _, t := range typs {
			fmt.Println(t.GenStruct())
		}
		//fmt.Println(out)
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.graphql-gen-go.yaml)")
	RootCmd.PersistentFlags().StringVar(&pkgName, "pkg", "main", "generated golang package name")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(".graphql-gen-go") // name of config file (without extension)
	viper.AddConfigPath("$HOME")           // adding home directory as first search path
	viper.AutomaticEnv()                   // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
