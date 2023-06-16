package keeper_test

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	erc20types "github.com/evmos/evmos/v9/x/erc20/types"

	inflationtypes "github.com/evmos/evmos/v9/x/inflation/types"
)

var erc20Denom = "erc20/0xdac17f958d2ee523a2206206994597c13d831ec7"

func (suite *KeeperTestSuite) TestConvertCoinToERC20FromPacket() {
	senderAddr := "oraie16yefkqyzshnrvz4sxzufndyfphnkajs9rmhrht"

	testCases := []struct {
		name     string
		malleate func() transfertypes.FungibleTokenPacketData
		transfer transfertypes.FungibleTokenPacketData
		expPass  bool
	}{
		{
			name: "error - invalid sender",
			malleate: func() transfertypes.FungibleTokenPacketData {
				return transfertypes.NewFungibleTokenPacketData("aevmos", "10", "", "")
			},
			expPass: false,
		},
		{
			name: "pass - is base denom",
			malleate: func() transfertypes.FungibleTokenPacketData {
				return transfertypes.NewFungibleTokenPacketData("aevmos", "10", senderAddr, "")
			},
			expPass: true,
		},
		{
			name: "pass - erc20 is disabled",
			malleate: func() transfertypes.FungibleTokenPacketData {
				metadata, pair := suite.setupRegisterCoin()
				suite.Require().NotNil(metadata)
				suite.Require().NotNil(pair)

				params := suite.app.Erc20Keeper.GetParams(suite.ctx)
				params.EnableErc20 = false
				suite.app.Erc20Keeper.SetParams(suite.ctx, params)
				return transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", senderAddr, "")
			},
			expPass: true,
		},
		{
			name: "pass - denom is not registered",
			malleate: func() transfertypes.FungibleTokenPacketData {
				metadata, pair := suite.setupRegisterCoin()
				suite.Require().NotNil(metadata)
				suite.Require().NotNil(pair)
				// Mint coins on account to simulate receiving ibc transfer
				sender, err := sdk.AccAddressFromBech32(senderAddr)
				suite.Require().NoError(err)
				coinEvmos := sdk.NewCoin(pair.Denom, sdk.NewInt(10))
				coins := sdk.NewCoins(coinEvmos)
				err = suite.app.BankKeeper.MintCoins(suite.ctx, inflationtypes.ModuleName, coins)
				suite.Require().NoError(err)
				err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, inflationtypes.ModuleName, sender, coins)
				suite.Require().NoError(err)
				return transfertypes.NewFungibleTokenPacketData(metadata.Base, "10", senderAddr, "")
			},
			expPass: true,
		},
		{
			name: "pass - denom is registered and has available balance",
			malleate: func() transfertypes.FungibleTokenPacketData {
				metadata, pair := suite.setupRegisterCoin()
				suite.Require().NotNil(metadata)
				suite.Require().NotNil(pair)

				sender, err := sdk.AccAddressFromBech32(senderAddr)
				suite.Require().NoError(err)

				// Mint coins on account to simulate receiving ibc transfer
				coinEvmos := sdk.NewCoin(pair.Denom, sdk.NewInt(10))
				coins := sdk.NewCoins(coinEvmos)
				err = suite.app.BankKeeper.MintCoins(suite.ctx, inflationtypes.ModuleName, coins)
				suite.Require().NoError(err)
				err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, inflationtypes.ModuleName, sender, coins)
				suite.Require().NoError(err)

				return transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", senderAddr, "")
			},
			expPass: true,
		},
		{
			name: "error - denom is registered but has no available balance",
			malleate: func() transfertypes.FungibleTokenPacketData {
				metadata, pair := suite.setupRegisterCoin()
				suite.Require().NotNil(metadata)
				suite.Require().NotNil(pair)

				return transfertypes.NewFungibleTokenPacketData(pair.Denom, "10", senderAddr, "")
			},
			expPass: false,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.mintFeeCollector = true
			suite.SetupTest() // reset

			transfer := tc.malleate()

			err := suite.app.Erc20Keeper.ConvertCoinToERC20FromPacket(suite.ctx, transfer)
			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestConvertLegacyToCurrentDenomMap() {
	recipient := "oraie16yefkqyzshnrvz4sxzufndyfphnkajs9rmhrht"
	erc20Contract := common.HexToAddress("0xdac17f958d2ee523a2206206994597c13d831ec7")

	testCases := []struct {
		name     string
		malleate func() sdk.Coin
		transfer transfertypes.FungibleTokenPacketData
		expPass  bool
	}{
		{
			name: "passed - legacy denom not registered should return nil pair id",
			malleate: func() sdk.Coin {
				return sdk.NewCoin("legacy", sdk.NewInt(1))
			},
			expPass: false,
		},
		{
			name: "passed - Cannot find current token pair should return nil pair id",
			malleate: func() sdk.Coin {
				coin := sdk.NewCoin("legacy", sdk.NewInt(1))
				suite.app.Erc20Keeper.SetLegacyDenomMap(suite.ctx, coin.Denom, erc20Contract.Bytes())
				suite.app.Erc20Keeper.SetERC20Map(suite.ctx, erc20Contract, []byte{1})
				return coin
			},
			expPass: false,
		},
		{
			name: "passed - ok",
			malleate: func() sdk.Coin {
				coin := sdk.NewCoin("legacy", sdk.NewInt(1))
				suite.app.Erc20Keeper.SetLegacyDenomMap(suite.ctx, coin.Denom, erc20Contract.Bytes())
				suite.app.Erc20Keeper.SetERC20Map(suite.ctx, erc20Contract, []byte{1})
				pair := erc20types.TokenPair{
					Erc20Address:  erc20Contract.Hex(),
					Denom:         "current",
					Enabled:       true,
					ContractOwner: erc20types.OWNER_MODULE,
				}
				suite.app.Erc20Keeper.SetTokenPair(suite.ctx, pair)
				// mint legacy coin so we can burn it later in the logic
				suite.app.BankKeeper.MintCoins(suite.ctx, erc20types.ModuleName, sdk.NewCoins(coin))
				return coin
			},
			expPass: false,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.mintFeeCollector = true
			suite.SetupTest() // reset

			coin := tc.malleate()

			coinDenom := suite.app.Erc20Keeper.ConvertLegacyToCurrentDenomMap(suite.ctx, coin, sdk.AccAddress(recipient))
			if tc.expPass {
				suite.Require().NotEmpty(coinDenom)
				suite.Require().Equal(coinDenom, "current")
				recipientBalance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(recipient), coinDenom)
				suite.Require().Equal(recipientBalance, sdk.NewCoin(coinDenom, coin.Amount))
			} else {
				suite.Require().Equal(coinDenom, "legacy")
			}
		})
	}
}
