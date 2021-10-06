/*
Copyright (c) 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This file contains the implementation of the `zap` encoder that generates the log format that we
// use.

package logging

import (
	"encoding/base64"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// EncoderName is the name used to register and lookup the encoder.
const EncoderName = "ocm"

func init() {
	// Register the encoder:
	zap.RegisterEncoder(EncoderName, NewEncoder)
}

// Encoder is an implementation of the zpcore.Encoder interface that generates messages in the
// format that we use, something like this:
//
// 2021-06-22T11:33:51Z INFO osd/upgrade_policy_worker.go:163
// [id='c0fe8be7-ee19-4c64-b8a6-6977cd234f25'] No upgrade policies found
type Encoder struct {
	objectEncoder

	// Pool of buffers that will be used by this encoder and by all the encoders cloned
	// from it.
	pool buffer.Pool
}

// Make sure that we implement the interface.
var _ zapcore.Encoder = (*Encoder)(nil)

// NewEncoder creates a new encoder that generates messages using in the format that we use.
func NewEncoder(config zapcore.EncoderConfig) (encoder zapcore.Encoder, err error) {
	pool := buffer.NewPool()
	encoder = &Encoder{
		objectEncoder: objectEncoder{
			buf: pool.Get(),
		},
		pool: pool,
	}
	return
}

// Clone is part of the zapcore.Encoder interface.
func (e *Encoder) Clone() zapcore.Encoder {
	// Create a copy of the context buffer, so that it can be modified by the cloned encoder
	// without affecting the original:
	buf := e.pool.Get()
	buf.Write(e.buf.Bytes())

	// Create the clone, using the copy of the context buffer but sharing the configuration and
	// the pool of buffers:
	return &Encoder{
		objectEncoder: objectEncoder{
			buf: buf,
		},
		pool: e.pool,
	}
}

// EncodeEntry is part of the zapcore.Encoder interface.
func (e *Encoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (lineBuf *buffer.Buffer,
	err error) {
	lineBuf = e.pool.Get()
	lineEnc := &objectEncoder{
		buf: lineBuf,
	}
	lineBuf.AppendTime(entry.Time.UTC(), time.RFC3339)
	lineBuf.AppendString(" ")
	lineBuf.AppendString(entry.Level.CapitalString())
	lineBuf.AppendString(" ")
	lineBuf.AppendString(entry.Caller.TrimmedPath())
	lineBuf.AppendString(" ")
	lineBuf.AppendString("[id='")
	id := ""
	for _, field := range fields {
		if field.Key == "id" {
			id = field.String
			break
		}
	}
	lineBuf.AppendString(id)
	lineBuf.AppendString("']")
	lineBuf.AppendString(" ")
	lineBuf.AppendString(entry.Message)
	if e.buf.Len() > 0 {
		lineBuf.AppendString(" ")
		lineBuf.Write(e.buf.Bytes())
	}
	for _, field := range fields {
		if field.Key != "id" {
			field.AddTo(lineEnc)
		}
	}
	lineBuf.AppendString("\n")
	if entry.Stack != "" {
		lineBuf.AppendString(entry.Stack)
		lineBuf.AppendString("\n")
	}
	return
}

// objectEncoder is used internally to encode fields to a buffer.
type objectEncoder struct {
	buf *buffer.Buffer
}

// Make sure that we implement the interface:
var _ zapcore.ObjectEncoder = (*objectEncoder)(nil)

// AddArray is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	encoder := &primitiveEncoder{
		buf: e.buf,
	}
	return marshaler.MarshalLogArray(encoder)
}

// AddObject is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	e.addKey(key)
	return marshaler.MarshalLogObject(e)
}

// AddBinary is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddBinary(key string, value []byte) {
	e.addKey(key)
	e.buf.AppendString(base64.StdEncoding.EncodeToString(value))
}

// AddByteString is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddByteString(key string, value []byte) {
	e.addKey(key)
	e.buf.Write(value)
}

// AddBool is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddBool(key string, value bool) {
	e.addKey(key)
	e.buf.AppendBool(value)
}

// AddComplex128 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddComplex128(key string, value complex128) {
	e.addKey(key)
	e.buf.AppendString(fmt.Sprintf("%v", value))
}

// AddComplex64 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddComplex64(key string, value complex64) {
	e.addKey(key)
	e.buf.AppendString(fmt.Sprintf("%v", value))
}

// AddDuration is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddDuration(key string, value time.Duration) {
	e.addKey(key)
	e.buf.AppendString(value.String())
}

// AddFloat64 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddFloat64(key string, value float64) {
	e.addKey(key)
	e.buf.AppendFloat(value, 64)
}

// AddFloat32 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddFloat32(key string, value float32) {
	e.addKey(key)
	e.buf.AppendFloat(float64(value), 32)
}

// AddInt is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddInt(key string, value int) {
	e.addKey(key)
	e.buf.AppendInt(int64(value))
}

// AddInt64 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddInt64(key string, value int64) {
	e.addKey(key)
	e.buf.AppendInt(value)
}

// AddInt32 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddInt32(key string, value int32) {
	e.addKey(key)
	e.buf.AppendInt(int64(value))
}

// AddInt16 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddInt16(key string, value int16) {
	e.addKey(key)
	e.buf.AppendInt(int64(value))
}

// AddInt8 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddInt8(key string, value int8) {
	e.addKey(key)
	e.buf.AppendInt(int64(value))
}

// AddString is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddString(key, value string) {
	e.addKey(key)
	e.buf.AppendString(value)
}

// AddTime is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddTime(key string, value time.Time) {
	e.addKey(key)
	e.buf.AppendTime(value.UTC(), time.RFC3339)
}

// AddUint is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUint(key string, value uint) {
	e.addKey(key)
	e.buf.AppendUint(uint64(value))
}

// AddUint64 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUint64(key string, value uint64) {
	e.addKey(key)
	e.buf.AppendUint(value)
}

// AddUint32 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUint32(key string, value uint32) {
	e.addKey(key)
	e.buf.AppendUint(uint64(value))
}

// AddUint16 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUint16(key string, value uint16) {
	e.addKey(key)
	e.buf.AppendUint(uint64(value))
}

// AddUint8 is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUint8(key string, value uint8) {
	e.addKey(key)
	e.buf.AppendUint(uint64(value))
}

// AddArray is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddUintptr(key string, value uintptr) {
	e.addKey(key)
	e.buf.AppendString(fmt.Sprintf("0x%016x", value))
}

// AddArray is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) AddReflected(key string, value interface{}) error {
	e.addKey(key)
	e.buf.AppendString(fmt.Sprintf("%v", value))
	return nil
}

// AddArray is part of the zapcore.ObjectEncoder interface.
func (e *objectEncoder) OpenNamespace(key string) {
	// We don't use this.
}

func (e *objectEncoder) addKey(key string) {
	if e.buf.Len() > 0 {
		e.buf.AppendString(" ")
	}
	e.buf.AppendString(key)
	e.buf.AppendString("=")
}

// primitiveEncoder is used internally to encode primitive types.
type primitiveEncoder struct {
	buf *buffer.Buffer
}

// Make sure that we implement the interface:
var _ zapcore.PrimitiveArrayEncoder = (*primitiveEncoder)(nil)

// AppendBool is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendBool(value bool) {
	e.buf.AppendBool(value)
}

// AppendByteString is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendByteString(value []byte) {
	e.buf.Write(value)
}

// AppendComplex128 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendComplex128(value complex128) {
	e.buf.AppendString(fmt.Sprintf("%v", value))
}

// AppendComples64 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendComplex64(value complex64) {
	e.buf.AppendString(fmt.Sprintf("%v", value))
}

// AppendFloat64 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendFloat64(value float64) {
	e.buf.AppendFloat(value, 64)
}

// AppendFloat32 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendFloat32(value float32) {
	e.buf.AppendFloat(float64(value), 32)
}

// AppendInt is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendInt(value int) {
	e.buf.AppendInt(int64(value))
}

// AppendInt64 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendInt64(value int64) {
	e.buf.AppendInt(value)
}

// AppendInt32 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendInt32(value int32) {
	e.buf.AppendInt(int64(value))
}

// AppendInt16 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendInt16(value int16) {
	e.buf.AppendInt(int64(value))
}

// AppendInt8 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendInt8(value int8) {
	e.buf.AppendInt(int64(value))
}

// AppendString is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendString(value string) {
	e.buf.AppendString(value)
}

// AppendUint is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUint(value uint) {
	e.buf.AppendUint(uint64(value))
}

// AppendUint64 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUint64(value uint64) {
	e.buf.AppendUint(value)
}

// AppendUint32 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUint32(value uint32) {
	e.buf.AppendUint(uint64(value))
}

// AppendUint16 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUint16(value uint16) {
	e.buf.AppendUint(uint64(value))
}

// AppendUint8 is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUint8(value uint8) {
	e.buf.AppendUint(uint64(value))
}

// AppendUintptr is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendUintptr(value uintptr) {
	e.buf.AppendString(fmt.Sprintf("0x08%x", value))
}

// AppendDuration is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendDuration(value time.Duration) {
	e.buf.AppendString(value.String())
}

// AppendTime is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendTime(value time.Time) {
	e.buf.AppendTime(value.UTC(), time.RFC3339)
}

// AppendArray is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendArray(marshaler zapcore.ArrayMarshaler) error {
	return marshaler.MarshalLogArray(e)
}

// AppendObject is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendObject(marshaler zapcore.ObjectMarshaler) error {
	return nil
}

// AppendReflected is part of the zapcore.ArrayEncoder interface.
func (e *primitiveEncoder) AppendReflected(value interface{}) error {
	e.buf.AppendString(fmt.Sprintf("%v", value))
	return nil
}
