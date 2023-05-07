package commands

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/light"
	"github.com/tendermint/tendermint/node"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/statesync"
	"github.com/tendermint/tendermint/store"
)

// BootstrapStateCmd is a cobra command that bootstrap cometbft state in an arbitrary block height using light client
var BootstrapStateCmd = &cobra.Command{
	Use:   "bootstrap-state height",
	Short: "Bootstrap cometbft state in an arbitrary block height using light client",
	Long: `
Bootstrap cometbft state in an arbitrary block height using light client
`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		height, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return err
		}
		return bootstrapStateCmd(height)
	},
}

func bootstrapStateCmd(height uint64) error {
	ctx := context.Background()

	blockStoreDB, err := node.DefaultDBProvider(&node.DBContext{ID: "blockstore", Config: config})
	if err != nil {
		return err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	stateDB, err := node.DefaultDBProvider(&node.DBContext{ID: "state", Config: config})
	if err != nil {
		return err
	}
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: config.Storage.DiscardABCIResponses,
	})

	genState, _, err := node.LoadStateFromDBOrGenesisDocProvider(stateDB, node.DefaultGenesisDocProviderFunc(config))
	if err != nil {
		return err
	}

	stateProvider, err := statesync.NewLightClientStateProvider(
		ctx,
		genState.ChainID, genState.Version, genState.InitialHeight,
		config.StateSync.RPCServers, light.TrustOptions{
			Period: config.StateSync.TrustPeriod,
			Height: config.StateSync.TrustHeight,
			Hash:   config.StateSync.TrustHashBytes(),
		}, logger.With("module", "light"))
	if err != nil {
		return fmt.Errorf("failed to set up light client state provider: %w", err)
	}

	state, err := stateProvider.State(ctx, height)
	if err != nil {
		return err
	}

	commit, err := stateProvider.Commit(ctx, height)
	if err != nil {
		return err
	}

	if err := stateStore.Bootstrap(state); err != nil {
		return err
	}

	return blockStore.SaveSeenCommit(state.LastBlockHeight, commit)
}
