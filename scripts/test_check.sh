#!/usr/bin/env bash
echo "Exit code: $(cat exit-code)" >> test.log; 
grep -A10 'panic: test timed out' test.log || true; 
grep -A1 -- '--- SKIP:' test.log || true; 
grep -A1 -- '--- FAIL:' test.log || true; 
grep '^FAIL' test.log || true;
exit_code=`cat exit-code`
echo $exit_code 
if [ ${exit_code} == "0" ]; then echo "PASS" ; exit 0 ; else echo "TESTS FAILED"; exit 1 ; fi 

