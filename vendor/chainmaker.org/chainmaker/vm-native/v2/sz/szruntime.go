package sz

import (
	"chainmaker.org/chainmaker/pb-go/v2/common"
	"chainmaker.org/chainmaker/protocol/v2"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"strconv"
)

const (
	bizId            = "bizId"
	timestamp        = "timestamp"
	reqSign          = "reqSign"
	wPars            = "wPars"
	rPars            = "rPars"
	mutableContent   = "mutableContent"
	immutableContent = "immutableContent"
	nonce            = "nonce"
	txID             = "txId"
	businessType     = "businessType"
	chainName        = "chainName"
)

type Runtime struct {
	Log          protocol.Logger
	ContractName string
}

type value struct {
	TxId  string
	Nonce int
}

type szPair struct {
	BizId            []byte `json:"bizId"`
	Timestamp        []byte `json:"timestamp"`
	ReqSign          []byte `json:"reqSign"`
	WPars            []byte `json:"wPars"`
	RPars            []byte `json:"rPars"`
	MutableContent   []byte `json:"mutableContent"`
	ImmutableContent []byte `json:"immutableContent"`
	Nonce            []byte `json:"nonce"`
	TxId             []byte `json:"txId"`
	BusinessType     []byte `json:"businessType"`
	ChainName        []byte `json:"chainName"`
}

func (r *Runtime) Save(txSimContext protocol.TxSimContext, params map[string][]byte) (result []byte, err error) {

	//if params == nil {
	//	r.Log.Errorf("sz save fail %s contract:%s", ErrParamsEmpty, r.ContractName)
	//	return nil, ErrParamsEmpty
	//}
	//
	//kList := []string{
	//	bizId,
	//	timestamp,
	//	reqSign,
	//	wPars,
	//	rPars,
	//	businessType,
	//	chainName,
	//}
	//
	//if err = r.validateParams(params, kList); err != nil {
	//	errMsg := fmt.Sprintf("sz save fail valid params contract:%s, err:%s", r.ContractName, err.Error())
	//	r.Log.Error(errMsg)
	//	return nil, errors.New(errMsg)
	//}
	//
	//_, okMutableContent := params[mutableContent]
	//_, okImmutableContent := params[immutableContent]
	//
	//if !okMutableContent && !okImmutableContent {
	//	errMsg := fmt.Sprintf("sz save fail contract:%s, param err:%s",
	//		r.ContractName, ErrSomeParamEmpty("okMutableContent or okImmutableContent"))
	//	return nil, errors.New(errMsg)
	//}

	simContextKey := r.getSimContextKey(params[bizId], params[businessType])

	//res, err := txSimContext.Get(r.ContractName, []byte(simContextKey))
	//if err != nil {
	//	errMsg := fmt.Sprintf("sz save fail txSimContext get err:%s, contract:%s, simContextKey:%s",
	//		err.Error(), r.ContractName, simContextKey)
	//	r.Log.Error(errMsg)
	//	return nil, errors.New(errMsg)
	//}
	//
	//if res != nil {
	//	errMsg := fmt.Sprintf("sz save fail txSimContext get tx existed contract:%s,simContextKey:%s",
	//		r.ContractName, simContextKey)
	//	r.Log.Error(errMsg)
	//	return nil, errors.New(errMsg)
	//}

	txId := txSimContext.GetTx().Payload.TxId

	txSimContextValue := []*value{
		{
			TxId:  txId,
			Nonce: 0,
		},
	}

	valueByte, err := json.Marshal(txSimContextValue)

	if err != nil {
		errMsg := fmt.Sprintf("sz save fail json marshal err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	err = txSimContext.Put(r.ContractName, []byte(simContextKey), valueByte)

	if err != nil {
		errMsg := fmt.Sprintf("sz save fail txSimContext put err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	r.Log.DebugDynamic(func() string {
		return fmt.Sprintf("sz save success contract:%s,simContextKey:%s,txId:%s", r.ContractName, simContextKey, txId)
	})
	return []byte(txId), nil
}

func (r *Runtime) Update(txSimContext protocol.TxSimContext, params map[string][]byte) (result []byte, err error) {

	//if params == nil {
	//	r.Log.Errorf("sz update fail %s contract:%s", ErrParamsEmpty, r.ContractName)
	//	return nil, ErrParamsEmpty
	//}
	//
	//kList := []string{
	//	bizId,
	//	timestamp,
	//	reqSign,
	//	wPars,
	//	rPars,
	//	businessType,
	//	chainName,
	//	mutableContent,
	//}
	//
	//err = r.validateParams(params, kList)
	//
	//if err != nil {
	//	errMsg := fmt.Sprintf("sz update fail valid params contract:%s, err:%s", r.ContractName, err.Error())
	//	r.Log.Error(errMsg)
	//	return nil, errors.New(errMsg)
	//}

	simContextKey := []byte(r.getSimContextKey(params[bizId], params[businessType]))
	valueByte, err := txSimContext.Get(r.ContractName, simContextKey)
	if err != nil {
		errMsg := fmt.Sprintf("sz update fail txSimContext get err:%s contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	if valueByte == nil {
		errMsg := fmt.Sprintf("sz update fail txSimContext get tx not existed contract:%s,simContextKey:%s",
			r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	simContextValue := make([]*value, 0)

	if err = json.Unmarshal(valueByte, &simContextValue); err != nil {
		errMsg := fmt.Sprintf("sz update fail json unmarshal err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	// get the last tx value
	lastNonce := simContextValue[len(simContextValue)-1].Nonce

	nonce, okNonce := params[nonce]
	// if nonce existed, the nonce should be more than the last
	var noncePair int

	strNonce := string(nonce)
	if okNonce && strNonce != "" {

		nonceInt := 0
		nonceInt, err = strconv.Atoi(strNonce)
		if err != nil {
			errMsg := fmt.Sprintf("sz update fail strconv Atoi err:%s, contract:%s,simContextKey:%s",
				err, r.ContractName, simContextKey)
			r.Log.Error(errMsg)
			return nil, errors.New(errMsg)
		}

		if lastNonce >= nonceInt {
			errMsg := fmt.Sprintf("sz update fail nonce invalid request nonce should be more than the last, "+
				"contract:%s,simContextKey:%s", r.ContractName, simContextKey)
			r.Log.Error(errMsg)
			return nil, errors.New(errMsg)
		}
		noncePair = nonceInt

	} else {
		noncePair = lastNonce + 1
		// replace the last
	}

	txId := txSimContext.GetTx().Payload.TxId

	simContextValue = append(simContextValue, &value{
		TxId:  txId,
		Nonce: noncePair,
	})

	valueByte, err = json.Marshal(simContextValue)

	if err != nil {
		errMsg := fmt.Sprintf("sz update fail nonce json marshal err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	err = txSimContext.Put(r.ContractName, simContextKey, valueByte)
	if err != nil {
		errMsg := fmt.Sprintf("sz update fail txSimContext put err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	res, err := json.Marshal(struct {
		TxId  []byte
		Nonce []byte
	}{
		TxId:  []byte(txId),
		Nonce: []byte(strconv.Itoa(noncePair)),
	})

	if err != nil {
		errMsg := fmt.Sprintf("sz update fail json marshal err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	r.Log.DebugDynamic(func() string {
		return fmt.Sprintf("sz update success contract:%s,simContextKey:%s,txId:%s", r.ContractName, simContextKey, txId)
	})
	return res, nil
}

func (r *Runtime) Get(txSimContext protocol.TxSimContext, params map[string][]byte) (result []byte, err error) {

	if params == nil {
		r.Log.Errorf("sz get fail %s contract:%s", ErrParamsEmpty, r.ContractName)
		return nil, ErrParams
	}

	kList := []string{
		bizId,
		businessType,
		chainName,
	}

	err = r.validateParams(params, kList)

	if err != nil {
		errMsg := fmt.Sprintf("sz get fail valid params contract:%s,err:%s", r.ContractName, err.Error())
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	simContextKey := []byte(r.getSimContextKey(params[bizId], params[businessType]))

	valueByte, err := txSimContext.Get(r.ContractName, simContextKey)
	if err != nil {
		errMsg := fmt.Sprintf("sz get fail txSimContext get err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	if valueByte == nil {
		warnMsg := fmt.Sprintf("sz get warn txSimContext get result is empty,contract:%s,simContextKey:%s",
			r.ContractName, simContextKey)
		r.Log.Warnf(warnMsg)

		return nil, nil
	}

	simContextValue := make([]*value, 0)

	if err = json.Unmarshal(valueByte, &simContextValue); err != nil {
		errMsg := fmt.Sprintf("sz get fail json unmarshal err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	// if the tx value only one, the last equal the first
	if len(simContextValue) == 1 {
		var pair *szPair
		pair, err = r.getSzPairByTxId(txSimContext, simContextValue[0].TxId)
		if err != nil {
			errMsg := fmt.Sprintf("sz get fail getSzPairByTxId tx value only one err:%s,contract:%s,simContextKey:%s",
				err.Error(), r.ContractName, simContextKey)
			r.Log.Error(errMsg)
			return nil, errors.New(errMsg)
		}
		r.Log.Debugf("sz get pair:%v", pair)
		pair.Nonce = []byte(strconv.Itoa(simContextValue[0].Nonce)) // TODO
		pair.TxId = []byte(simContextValue[0].TxId)

		var result []byte
		result, err = json.Marshal(pair)

		if err != nil {
			errMsg := fmt.Sprintf("sz get fail json marshal err:%s,contract:%s,simContextKey:%s",
				err.Error(), r.ContractName, simContextKey)
			r.Log.Error(errMsg)
			return nil, errors.New(errMsg)
		}
		r.Log.Debugf("sz get success contract:%s,simContextKey:%s", r.ContractName, simContextKey)
		return result, nil
	}

	firstPair, err := r.getSzPairByTxId(txSimContext, simContextValue[0].TxId)
	if err != nil {
		errMsg := fmt.Sprintf("sz get fail getSzPairByTxId first err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	lastPair, err := r.getSzPairByTxId(txSimContext, simContextValue[len(simContextValue)-1].TxId)
	if err != nil {
		errMsg := fmt.Sprintf("sz get fail getSzPairByTxId last err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	// get the first immutableContent
	lastPair.ImmutableContent = firstPair.ImmutableContent
	lastPair.Nonce = []byte(strconv.Itoa(simContextValue[len(simContextValue)-1].Nonce)) // TODO
	lastPair.TxId = []byte(simContextValue[len(simContextValue)-1].TxId)

	resByte, err := json.Marshal(lastPair)

	if err != nil {
		errMsg := fmt.Sprintf("sz get fail json Marshal err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}
	r.Log.DebugDynamic(func() string {
		return fmt.Sprintf("sz get success contract:%s,simContextKey:%s", r.ContractName, simContextKey)
	})
	return resByte, nil
}

func (r *Runtime) TraceSource(txSimContext protocol.TxSimContext,
	params map[string][]byte) (valueByte []byte, err error) {

	if params == nil {
		r.Log.Errorf("sz trace fail source %s contract:%s", ErrParamsEmpty, r.ContractName)
		return nil, ErrParams
	}

	kList := []string{
		bizId,
		businessType,
		chainName,
	}

	err = r.validateParams(params, kList)

	if err != nil {
		errMsg := fmt.Sprintf("sz trace fail source valid params contract:%s, err:%s", r.ContractName, err.Error())
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	simContextKey := []byte(r.getSimContextKey(params[bizId], params[businessType]))

	valueByte, err = txSimContext.Get(r.ContractName, simContextKey)
	if err != nil {
		errMsg := fmt.Sprintf("sz trace fail source txSimContext get err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	if valueByte == nil {
		errMsg := fmt.Sprintf("sz trace warn source txSimContext get result is empty, contract:%s,simContextKey:%s",
			r.ContractName, simContextKey)
		r.Log.Warn(errMsg)
		return nil, nil
	}
	simContextValue := make([]*value, 0)

	if err = json.Unmarshal(valueByte, &simContextValue); err != nil {
		errMsg := fmt.Sprintf("sz trace fail json unmarshal err:%s, contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	pairs, err := r.getSzPairsByTxIds(txSimContext, simContextValue)

	if err != nil {
		errMsg := fmt.Sprintf("sz trace fail source get all tx value err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, errors.New(errMsg)
	}

	res, err := json.Marshal(pairs)

	if err != nil {
		errMsg := fmt.Sprintf("sz trace fail source json marshal err:%s,contract:%s,simContextKey:%s",
			err.Error(), r.ContractName, simContextKey)
		r.Log.Error(errMsg)
		return nil, err
	}

	r.Log.DebugDynamic(func() string {
		return fmt.Sprintf("sz trace source success contract:%s,simContextKey:%s", r.ContractName, simContextKey)
	})
	return res, nil
}

func (r *Runtime) validateParams(params map[string][]byte, kList []string) (err error) {

	for _, key := range kList {
		value, ok := params[key]
		if !ok || value == nil {
			return errors.New(ErrSomeParamEmpty(key))
		}
	}

	return nil
}

func (r *Runtime) getSimContextKey(bizId, businessType []byte) string {
	simContextKey := string(bizId) + string(businessType)
	return simContextKey
}

func (r *Runtime) getSzPairByTxId(txSimContext protocol.TxSimContext, txId string) (pair *szPair, err error) {

	store := txSimContext.GetBlockchainStore()
	tx, err := store.GetTx(txId)

	if err != nil {
		errMsg := fmt.Sprintf("store getTx err:%s, tx id:%s", err.Error(), txId)
		return nil, errors.New(errMsg)
	}

	if tx.Payload.Parameters == nil || len(tx.Payload.Parameters) == 0 {
		errMsg := fmt.Sprintf("tx Payload Parameters is nil, txid:%s", txId)
		return nil, errors.New(errMsg)
	}

	pair = r.switchParamsToSzPair(tx.Payload.GetParameters())

	return pair, nil
}

func (r *Runtime) switchParamsToSzPair(parameters []*common.KeyValuePair) *szPair {

	pair := &szPair{}
	for _, v := range parameters {

		if v.GetKey() == bizId {
			pair.BizId = v.GetValue()
		}
		if v.GetKey() == timestamp {
			pair.Timestamp = v.GetValue()
		}
		if v.GetKey() == reqSign {
			pair.ReqSign = v.GetValue()
		}
		if v.GetKey() == wPars {
			pair.WPars = v.GetValue()
		}
		if v.GetKey() == rPars {
			pair.RPars = v.GetValue()
		}
		if v.GetKey() == mutableContent {
			pair.MutableContent = v.GetValue()
		}
		if v.GetKey() == immutableContent {
			pair.ImmutableContent = v.GetValue()
		}
		if v.GetKey() == nonce {
			pair.Nonce = v.GetValue()
		}
		if v.GetKey() == txID {
			pair.TxId = v.GetValue()
		}
		if v.GetKey() == businessType {
			pair.BusinessType = v.GetValue()
		}
		if v.GetKey() == chainName {
			pair.ChainName = v.GetValue()
		}
	}
	return pair
}

func (r *Runtime) getSzPairsByTxIds(txSimContext protocol.TxSimContext,
	simContextValue []*value) (pairs []*szPair, err error) {
	r.Log.DebugDynamic(func() string {
		return "sz trace getSzPairsByTxIds"
	})

	store := txSimContext.GetBlockchainStore()
	num := len(simContextValue)
	pairs = make([]*szPair, num)

	group := &errgroup.Group{}
	for k, v := range simContextValue {

		key := k
		pairOne := v
		group.Go(func() error {
			tx, err := store.GetTx(pairOne.TxId)
			if err != nil {
				errMsg := fmt.Sprintf("store getTx err:%s, tx id:%s", err.Error(), pairOne.TxId)
				newErr := errors.New(errMsg)
				r.Log.Error(errMsg)
				return newErr
			}

			if tx.Payload.Parameters == nil || len(tx.Payload.Parameters) == 0 {
				errMsg := fmt.Sprintf("tx Payload Parameters is nil, txid:%s", pairOne.TxId)
				newErr := errors.New(errMsg)
				r.Log.Error(errMsg)
				return newErr
			}

			pair := r.switchParamsToSzPair(tx.Payload.Parameters)
			pair.Nonce = []byte(strconv.Itoa(pairOne.Nonce))
			pair.TxId = []byte(pairOne.TxId)
			pairs[key] = pair
			return nil
		})

	}

	if err := group.Wait(); err != nil {
		errMsg := fmt.Sprintf("get all tx err:%s", err.Error())
		return nil, errors.New(errMsg)
	}

	return pairs, nil
}
