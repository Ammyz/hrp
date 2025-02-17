## hrp run

run API test

### Synopsis

run yaml/json testcase files for API test

```
hrp run $path... [flags]
```

### Examples

```
  $ hrp run demo.json	# run specified json testcase file
  $ hrp run demo.yaml	# run specified yaml testcase file
  $ hrp run examples/	# run testcases in specified folder
```

### Options

```
      --continue-on-failure   continue running next step when failure occurs
  -h, --help                  help for run
  -p, --proxy-url string      set proxy url
  -s, --silent                disable logging request & response details
```

### SEE ALSO

* [hrp](hrp.md)	 - One-stop solution for HTTP(S) testing.

###### Auto generated by spf13/cobra on 8-Jan-2022
