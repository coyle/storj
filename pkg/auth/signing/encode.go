// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package signing

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
)

// EncodeOrderLimit encodes order limit into bytes for signing. Removes signature from serialized limit.
func EncodeOrderLimit(ctx context.Context, limit *pb.OrderLimit) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.OrderLimitSigning{}
	signing.SerialNumber = limit.SerialNumber
	signing.SatelliteId = limit.SatelliteId
	if limit.DeprecatedUplinkId != nil && !limit.DeprecatedUplinkId.IsZero() {
		signing.DeprecatedUplinkId = limit.DeprecatedUplinkId
	}
	if !limit.UplinkPublicKey.IsZero() {
		signing.UplinkPublicKey = &limit.UplinkPublicKey
	}
	signing.StorageNodeId = limit.StorageNodeId
	signing.PieceId = limit.PieceId
	signing.Limit = limit.Limit
	signing.Action = limit.Action
	if !limit.PieceExpiration.IsZero() {
		signing.PieceExpiration = &limit.PieceExpiration
	}
	if !limit.OrderExpiration.IsZero() {
		signing.OrderExpiration = &limit.OrderExpiration
	}
	if !limit.OrderCreation.IsZero() {
		signing.OrderCreation = &limit.OrderCreation
	}
	signing.SatelliteAddress = limit.SatelliteAddress

	return proto.Marshal(&signing)
}

// EncodeOrder encodes order into bytes for signing. Removes signature from serialized order.
func EncodeOrder(ctx context.Context, order *pb.Order) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.OrderSigning{}
	signing.SerialNumber = order.SerialNumber
	signing.Amount = order.Amount

	return proto.Marshal(&signing)
}

// EncodePieceHash encodes piece hash into bytes for signing. Removes signature from serialized hash.
func EncodePieceHash(ctx context.Context, hash *pb.PieceHash) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)

	// protobuf has problems with serializing types with nullable=false
	// this uses a different message for signing, such that the rest of the code
	// doesn't have to deal with pointers for those particular fields.

	signing := pb.PieceHashSigning{}
	signing.PieceId = hash.PieceId
	signing.Hash = hash.Hash
	signing.PieceSize = hash.PieceSize
	if !hash.Timestamp.IsZero() {
		signing.Timestamp = &hash.Timestamp
	}
	return proto.Marshal(&signing)
}

// EncodeVoucher encodes voucher into bytes for signing.
func EncodeVoucher(ctx context.Context, voucher *pb.Voucher) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := voucher.SatelliteSignature
	voucher.SatelliteSignature = nil
	out, err := proto.Marshal(voucher)
	voucher.SatelliteSignature = signature
	return out, err
}

// EncodeStreamID encodes stream ID into bytes for signing.
func EncodeStreamID(ctx context.Context, streamID *pb.SatStreamID) (_ []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := streamID.SatelliteSignature
	// TODO verify if that can cause race
	streamID.SatelliteSignature = nil
	out, err := proto.Marshal(streamID)
	streamID.SatelliteSignature = signature
	return out, err
}
