package main

import (
	"encoding/json"
	"fmt"
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	// Load config
	var configPath string
	var config giga.Config

	LoadConfig(configPath, &config)

	// define root command
	rootCmd := &cobra.Command{
		Use: "gigawallet",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(0)
		},
	}

	// Add flags for each configuration option
	rootCmd.PersistentFlags().StringVar(&config.Gigawallet.ServiceName, "service-name", "", "Service name")
	rootCmd.PersistentFlags().StringVar(&config.Gigawallet.ServiceDomain, "service-domain", "", "Service domain")
	rootCmd.PersistentFlags().StringVar(&config.Gigawallet.ServiceIconURL, "service-icon-url", "", "Service icon URL")
	rootCmd.PersistentFlags().StringVar(&config.Gigawallet.ServiceKeyHash, "service-key-hash", "", "Service key hash")
	rootCmd.PersistentFlags().StringVar(&config.Gigawallet.Dogecoind, "dogecoind", "", "Dogecoind")
	rootCmd.PersistentFlags().IntVar(&config.Gigawallet.ConfirmationsNeeded, "confirmations-needed", 0, "Confirmations needed")
	rootCmd.PersistentFlags().StringVar(&config.WebAPI.Port, "webapi-port", "", "Web API port")
	rootCmd.PersistentFlags().StringVar(&config.WebAPI.Bind, "webapi-bind", "", "Web API bind")
	rootCmd.PersistentFlags().StringVar(&config.Store.DBFile, "store-db-file", "", "Store DB file")
	// ...
	// Bind flags to config fields
	viper.BindPFlags(rootCmd.PersistentFlags())

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the GigaWallet server",
		Run: func(cmd *cobra.Command, args []string) {
			Server(config)
		},
	}

	configCmd := &cobra.Command{
		Use:   "showconf",
		Short: "Print the config state and exit",
		Run: func(cmd *cobra.Command, args []string) {
			o, _ := json.MarshalIndent(config, ">", " ")
			fmt.Println(string(o))
			os.Exit(0)
		},
	}

	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(configCmd)

	// Execute the Cobra command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}

}

func LoadConfig(configPath string, config *giga.Config) {

	configFileName, set := os.LookupEnv("GIGA_ENV")
	if set {
		viper.SetConfigName(configFileName)
	} else {
		viper.SetConfigName("config")
	}

	// Set config file name and search paths
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gigawallet/")
	viper.AddConfigPath("$HOME/.gigawallet")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("failed to find config file: ", err)
		os.Exit(1)
	}

	if err := viper.Unmarshal(&config); err != nil {
		panic(fmt.Errorf("failed to unmarshal config: %s", err))
	}
}
