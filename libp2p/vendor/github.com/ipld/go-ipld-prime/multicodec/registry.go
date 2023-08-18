package multicodec

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/codec"
)

// Registry is a structure for storing mappings of multicodec indicator numbers to codec.Encoder and codec.Decoder functions.
//
// The most typical usage of this structure is in combination with a codec.LinkSystem.
// For example, a linksystem using CIDs and a custom multicodec registry can be constructed
// using cidlink.LinkSystemUsingMulticodecRegistry.
//
// Registry includes no mutexing.  If using Registry in a concurrent context, you must handle synchronization yourself.
// (Typically, it is recommended to do initialization earlier in a program, before fanning out goroutines;
// this avoids the need for mutexing overhead.)
//
// go-ipld also has a default registry, which has the same methods as this structure, but are at package scope.
// Some systems, like cidlink.DefaultLinkSystem, will use this default registry.
// However, this default registry is global to the entire program.
// This Registry type is for helping if you wish to make your own registry which does not share that global state.
//
// Multicodec indicator numbers are specified in
// https://github.com/multiformats/multicodec/blob/master/table.csv .
// You should not use indicator numbers which are not specified in that table
// (however, there is nothing in this implementation that will attempt to stop you, either; please behave).
type Registry struct {
	encoders map[uint64]codec.Encoder
	decoders map[uint64]codec.Decoder
}

func (r *Registry) ensureInit() {
	if r.encoders != nil {
		return
	}
	r.encoders = make(map[uint64]codec.Encoder)
	r.decoders = make(map[uint64]codec.Decoder)
}

// RegisterEncoder updates a simple map of multicodec indicator number to codec.Encoder function.
// The encoder functions registered can be subsequently looked up using LookupEncoder.
func (r *Registry) RegisterEncoder(indicator uint64, encodeFunc codec.Encoder) {
	r.ensureInit()
	if encodeFunc == nil {
		panic("not sensible to attempt to register a nil function")
	}
	r.encoders[indicator] = encodeFunc
}

// LookupEncoder yields a codec.Encoder function matching a multicodec indicator code number.
//
// To be available from this lookup function, an encoder must have been registered
// for this indicator number by an earlier call to the RegisterEncoder function.
func (r *Registry) LookupEncoder(indicator uint64) (codec.Encoder, error) {
	encodeFunc, exists := r.encoders[indicator]
	if !exists {
		return nil, fmt.Errorf("no encoder registered for multicodec code %d (0x%x)", indicator, indicator)
	}
	return encodeFunc, nil
}

// ListEncoders returns a list of multicodec indicators for which a codec.Encoder is registered.
// The list is in no particular order.
func (r *Registry) ListEncoders() []uint64 {
	encoders := make([]uint64, 0, len(r.encoders))
	for e := range r.encoders {
		encoders = append(encoders, e)
	}
	return encoders
}

// TODO(mvdan): turn most of these uint64s into multicodec.Code

// RegisterDecoder updates a simple map of multicodec indicator number to codec.Decoder function.
// The decoder functions registered can be subsequently looked up using LookupDecoder.
func (r *Registry) RegisterDecoder(indicator uint64, decodeFunc codec.Decoder) {
	r.ensureInit()
	if decodeFunc == nil {
		panic("not sensible to attempt to register a nil function")
	}
	r.decoders[indicator] = decodeFunc
}

// LookupDecoder yields a codec.Decoder function matching a multicodec indicator code number.
//
// To be available from this lookup function, an decoder must have been registered
// for this indicator number by an earlier call to the RegisterDecoder function.
func (r *Registry) LookupDecoder(indicator uint64) (codec.Decoder, error) {
	decodeFunc, exists := r.decoders[indicator]
	if !exists {
		return nil, fmt.Errorf("no decoder registered for multicodec code %d (0x%x)", indicator, indicator)
	}
	return decodeFunc, nil
}

// ListDecoders returns a list of multicodec indicators for which a codec.Decoder is registered.
// The list is in no particular order.
func (r *Registry) ListDecoders() []uint64 {
	decoders := make([]uint64, 0, len(r.decoders))
	for d := range r.decoders {
		decoders = append(decoders, d)
	}
	return decoders
}
