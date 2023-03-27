import hasVariableDeclarationsAndReferences from 'nomad-ui/utils/has-variable-declarations';
import { module, test } from 'qunit';

module(
  'Unit | Util | #hasVariableDeclarationsAndReferences - HCL2 Variable Matching',
  function () {
    test('should match "variable" declarations', function (assert) {
      const str = 'variable "datacenters" { type = list(string) }';
      assert.ok(
        hasVariableDeclarationsAndReferences(str),
        'The string contains a "variable" declaration'
      );
    });

    test('should match "var." references', function (assert) {
      const str = 'datacenters = var.datacenters';
      assert.ok(
        hasVariableDeclarationsAndReferences(str),
        'The string contains a "var." reference'
      );
    });

    test('should not match other words containing "variable" or "var" without the dot', function (assert) {
      const str = 'This is a variablerandomtext and varwithsometext.';
      assert.notOk(
        hasVariableDeclarationsAndReferences(str),
        'The string does not contain a variable declaration or reference'
      );
    });

    test('should not match an empty string', function (assert) {
      const str = '';
      assert.notOk(
        hasVariableDeclarationsAndReferences(str),
        'The string does not contain a variable declaration or reference'
      );
    });
  }
);
