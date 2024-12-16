package cmd

import (
	"context"
	"github.com/michaelpento.lv/mevbot/utils"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	debug   bool
)

var rootCmd = &cobra.Command{
	Use:   "mevbot",
	Short: "A CLI MEV bot for sandwich and frontrun attacks",
	Long: `A CLI MEV bot that monitors the mempool for profitable opportunities
and executes sandwich and frontrun attacks using flash loans and Flashbots.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mevbot.json)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")
}

func initConfig() {
	utils.InitLogger(debug)
}
