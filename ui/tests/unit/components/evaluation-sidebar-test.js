/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import DetailComponent from 'nomad-ui/components/evaluation-sidebar/detail';

module('Unit | Component | evaluation-sidebar/detail', function (hooks) {
  setupTest(hooks);

  test('evaluationJSON serializes the evaluation model and excludes Ember internals', function (assert) {
    // Mock an Ember Data model with internal properties that should be excluded
    const mockEvaluation = {
      id: 'eval-123',
      status: 'complete',
      // Ember Data internal properties that should NOT appear in JSON
      _internalModel: { /* internal state */ },
      store: { /* store reference */ },
      __data__: { /* internal data */ },
      serialize() {
        // serialize() should return only the clean data
        return {
          id: this.id,
          status: this.status,
        };
      },
    };

    const component = new DetailComponent(this.owner, {
      statechart: {
        state: {
          context: {
            evaluation: mockEvaluation,
          },
        },
      },
    });

    const result = component.evaluationJSON;

    // Verify only clean data is returned, no Ember internals
    assert.deepEqual(result, {
      id: 'eval-123',
      status: 'complete',
    }, 'evaluationJSON returns only serialized data without Ember internals');

    assert.notOk(result._internalModel, 'does not include _internalModel');
    assert.notOk(result.store, 'does not include store');
    assert.notOk(result.__data__, 'does not include __data__');
  });

  test('evaluationJSON falls back to toJSON and excludes Ember internals', function (assert) {
    // Mock an Ember Data model without serialize() but with toJSON()
    const mockEvaluation = {
      id: 'eval-456',
      status: 'pending',
      // Ember Data internal properties that should NOT appear in JSON
      _internalModel: { /* internal state */ },
      store: { /* store reference */ },
      toJSON() {
        // toJSON() should return only the clean data
        return {
          id: this.id,
          status: this.status,
        };
      },
    };

    const component = new DetailComponent(this.owner, {
      statechart: {
        state: {
          context: {
            evaluation: mockEvaluation,
          },
        },
      },
    });

    const result = component.evaluationJSON;

    // Verify only clean data is returned via toJSON fallback
    assert.deepEqual(result, {
      id: 'eval-456',
      status: 'pending',
    }, 'evaluationJSON falls back to toJSON and returns clean data');

    assert.notOk(result._internalModel, 'does not include _internalModel');
    assert.notOk(result.store, 'does not include store');
  });

  test('evaluationJSON returns empty object if no serialization method available (prevents exposing Ember internals)', function (assert) {
    // Mock an Ember Data model without serialize() or toJSON()
    // This simulates the bug scenario where model internals would be exposed
    const mockEvaluation = {
      id: 'eval-789',
      status: 'failed',
      // Ember Data internal properties that WOULD be exposed without serialization
      _internalModel: { /* internal state */ },
      store: { /* store reference */ },
      __data__: { /* internal data */ },
      // No serialize() or toJSON() methods
    };

    const component = new DetailComponent(this.owner, {
      statechart: {
        state: {
          context: {
            evaluation: mockEvaluation,
          },
        },
      },
    });

    const result = component.evaluationJSON;

    // Without serialization methods, return empty object as safe fallback
    // This prevents exposing Ember internals to JsonViewer
    assert.deepEqual(result, {}, 'evaluationJSON returns empty object as safe fallback');
    assert.notOk(result._internalModel, 'does not expose _internalModel');
    assert.notOk(result.store, 'does not expose store');
    assert.notOk(result.__data__, 'does not expose __data__');
  });

  test('evaluationJSON returns empty object when evaluation is null (safe handling)', function (assert) {
    const component = new DetailComponent(this.owner, {
      statechart: {
        state: {
          context: {
            evaluation: null,
          },
        },
      },
    });

    const result = component.evaluationJSON;

    // When evaluation is null/undefined, return empty object as safe fallback
    // This prevents errors and ensures JsonViewer always receives valid JSON
    assert.deepEqual(result, {}, 'evaluationJSON returns empty object when evaluation is null');
    assert.strictEqual(typeof result, 'object', 'result is an object');
    assert.ok(Object.keys(result).length === 0, 'result is empty');
  });
});