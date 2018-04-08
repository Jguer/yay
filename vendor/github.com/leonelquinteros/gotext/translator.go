/*
 * Copyright (c) 2018 DeineAgentur UG https://www.deineagentur.com. All rights reserved.
 * Licensed under the MIT License. See LICENSE file in the project root for full license information.
 */

package gotext

type Translator interface {
	ParseFile(f string)
	Parse(buf []byte)
	Get(str string, vars ...interface{}) string
	GetN(str, plural string, n int, vars ...interface{}) string
	GetC(str, ctx string, vars ...interface{}) string
	GetNC(str, plural string, n int, ctx string, vars ...interface{}) string
}
