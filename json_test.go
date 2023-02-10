package json

import (
	"encoding/json"
	"fmt"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// LuaInit holds globals available to callers of newLuaState().
const LuaInit = `
	json = require("json")

	-- Tests plain values and list-style tables.
	function assert_eq(got, want)
		if type(got) == "table" and type(want) == "table" then
			assert(#got == #want, string.format("got %d elements, wanted %d", #got, #want))

			for i, gotv in ipairs(got) do
				local wantv = want[i]
				assert_eq(gotv, wantv, "got[%d] = %q, wanted %q", gotv, wantv)
			end

			return
		end

		assert(got == want, string.format("got %q, wanted %q", got, want))
	end

	function assert_contains(got, want)
		assert(string.find(got, want),
			string.format("got %q, wanted it to contain %q", got, want))
	end
`

func TestRequire(t *testing.T) {
	const code = `
		local j = require("json")
		assert(type(j) == "table")
		assert(type(j.decode) == "function")
		assert(type(j.encode) == "function")
	`

	ls := lua.NewState()
	defer ls.Close()

	ls.PreloadModule("json", Loader)
	if err := ls.DoString(code); err != nil {
		t.Error(err)
	}
}

func TestEncodeValues(t *testing.T) {
	tcs := map[string]struct {
		expr, want string
	}{
		"true":         {"true", "true"},
		"false":        {"false", "false"},
		"integer":      {"42", "42"},
		"negative int": {"-10", "-10"},
		"float":        {"1.234", "1.234"},
		"nil":          {"nil", "null"},
		"empty list":   {"{}", "[]"},
		"number list":  {"{1, 2, 3}", "[1,2,3]"},
		"string list":  {"{'a', 'b', 'c'}", `["a","b","c"]`},
	}

	ls := newLuaState()
	defer ls.Close()

	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			code := fmt.Sprintf("assert_eq(json.encode(%s), %q)\n", tc.expr, tc.want)
			if err := ls.DoString(code); err != nil {
				t.Error(code, err)
			}
		})
	}
}

func TestEncodeErrors(t *testing.T) {
	const code = `
		local _, err = json.encode({1, 2, [10] = 3})
		assert_contains(err, "sparse array")

		local _, err = json.encode({1, 2, 3, name = "Tim"})
		assert_contains(err, "mixed or invalid key types")

		local _, err = json.encode({name = "Tim", [false] = 123})
		assert_contains(err, "mixed or invalid key types")
	`

	ls := newLuaState()
	defer ls.Close()

	if err := ls.DoString(code); err != nil {
		t.Error(code, err)
	}
}

func TestDecodeValues(t *testing.T) {
	tcs := map[string]struct {
		input, expr string
	}{
		"true":         {"true", "true"},
		"false":        {"false", "false"},
		"integer":      {"42", "42"},
		"negative int": {"-10", "-10"},
		"float":        {"1.234", "1.234"},
		"null":         {"null", "nil"},
		"empty list":   {"[]", "{}"},
		"number list":  {"[1, 2, 3]", "{1, 2, 3}"},
		"string list":  {`["a", "b", "c"]`, "{'a', 'b', 'c'}"},
	}

	ls := newLuaState()
	defer ls.Close()

	for name, tc := range tcs {
		tc := tc
		t.Run(name, func(t *testing.T) {
			code := fmt.Sprintf(`
				local got, err = json.decode(%q)
				if err ~= nil then error(err) end
				assert_eq(got, %s)
			`, tc.input, tc.expr)

			if err := ls.DoString(code); err != nil {
				t.Error(code, err)
			}
		})
	}
}

func TestIncorrectArgLens(t *testing.T) {
	const code = `
		local status, err = pcall(function() json.decode() end)
		assert_contains(err, "bad argument #1 to decode")

		local status, err = pcall(function() json.decode(1,2) end)
		assert_contains(err, "bad argument #1 to decode")

		local status, err = pcall(function() json.encode() end)
		assert_contains(err, "bad argument #1 to encode")

		local status, err = pcall(function() json.encode(1,2) end)
		assert_contains(err, "bad argument #1 to encode")
	`

	ls := newLuaState()
	defer ls.Close()

	if err := ls.DoString(code); err != nil {
		t.Error(code, err)
	}
}

func TestComplexCases(t *testing.T) {
	const code = `
		-- Table round-trip.
		local obj = {"a",1,"b",2,"c",3}
		local jsonStr = json.encode(obj)
		local jsonObj = json.decode(jsonStr)
		assert_eq(jsonObj, obj)

		-- Table round-trip.
		local obj = {name="Tim",number=12345}
		local jsonStr = json.encode(obj)
		local jsonObj = json.decode(jsonStr)
		assert_eq(jsonObj.name, obj.name)
		assert_eq(jsonObj.number, obj.number)

		-- Table round-trip.
		assert(json.decode(json.encode({person={name = "tim",}})).person.name == "tim")

		-- Recursion.
		local obj = {
			abc = 123,
			def = nil,
		}
		local obj2 = {
			obj = obj,
		}
		obj.obj2 = obj2
		assert(json.encode(obj) == nil)
	`

	ls := newLuaState()
	defer ls.Close()

	if err := ls.DoString(code); err != nil {
		t.Error(err)
	}
}

func TestDecodeValue_jsonNumber(t *testing.T) {
	s := lua.NewState()
	defer s.Close()

	v := DecodeValue(s, json.Number("124.11"))
	if v.Type() != lua.LTString || v.String() != "124.11" {
		t.Fatalf("expecting LString, got %T", v)
	}
}

func newLuaState() *lua.LState {
	// Initialize Lua.
	ls := lua.NewState()
	ls.PreloadModule("json", Loader)
	if err := ls.DoString(LuaInit); err != nil {
		panic(err)
	}

	return ls
}
