/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import jsonToHcl from 'nomad-ui/utils/json-to-hcl';
import { module, test } from 'qunit';

module('Unit | Utility | json-to-hcl', function () {
  test('it preserves strings that look like JSON arrays without adding extra quotes', function (assert) {
    // This tests the scenario where a variable with type=string
    // has a default value that looks like below:
    // variable "example" {
    //   type    = string
    //   default = "[\"dc1\"]"
    // }
    const input = { example: '["dc1"]' };
    const result = jsonToHcl(input);

    assert.equal(result, 'example=["dc1"]\n');
  });

  test('it converts regular string values to quoted HCL', function (assert) {
    const input = { name: 'my-job' };
    const result = jsonToHcl(input);

    assert.equal(result, 'name="my-job"\n');
  });

  test('it converts numeric values to HCL', function (assert) {
    const input = { count: 3 };
    const result = jsonToHcl(input);

    assert.equal(result, 'count=3\n');
  });

  test('it converts boolean values to HCL', function (assert) {
    const input = { enabled: true };
    const result = jsonToHcl(input);

    assert.equal(result, 'enabled=true\n');
  });

  test('it preserves JSON object strings', function (assert) {
    const input = { config: '{"key": "value"}' };
    const result = jsonToHcl(input);

    assert.equal(result, 'config={"key": "value"}\n');
  });

  test('it handles strings that look like incomplete JSON arrays', function (assert) {
    const input = { text: '[incomplete' };
    const result = jsonToHcl(input);

    // Should be treated as a regular string and quoted
    assert.equal(result, 'text="[incomplete"\n');
  });

  test('it handles the case where a string variable contains invalid JSON-like syntax from CLI', function (assert) {
    // When variable has type=string, HCL2 treats it as literal string
    const input = { example: '[dc1,dc2]' };
    const result = jsonToHcl(input);

    // Should be quoted as a string since it's not valid JSON
    assert.equal(result, 'example="[dc1,dc2]"\n');
  });
});
