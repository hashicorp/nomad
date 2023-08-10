/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import cleanWhitespace from '../../utils/clean-whitespace';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | job diff', function (hooks) {
  setupRenderingTest(hooks);

  const commonTemplate = hbs`
    <div class="boxed-section">
      <div class="boxed-section-body is-dark">
        <JobDiff @diff={{diff}} />
      </div>
    </div>
  `;

  test('job field diffs', async function (assert) {
    assert.expect(5);

    this.set('diff', {
      ID: 'test-case-1',
      Type: 'Edited',
      Objects: null,
      Fields: [
        field('Removed Field', 'deleted', 12),
        field('Added Field', 'added', 'Foobar'),
        field('Edited Field', 'edited', 512, 256),
      ],
    });

    await render(commonTemplate);

    assert.equal(
      findAll('[data-test-diff-section-label]').length,
      5,
      'A section label for each line, plus one for the group'
    );
    assert.equal(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="field"][data-test-diff-field="added"]'
        ).textContent
      ),
      '+ Added Field: "Foobar"',
      'Added field is rendered correctly'
    );
    assert.equal(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="field"][data-test-diff-field="edited"]'
        ).textContent
      ),
      '+/- Edited Field: "256" => "512"',
      'Edited field is rendered correctly'
    );
    assert.equal(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="field"][data-test-diff-field="deleted"]'
        ).textContent
      ),
      '- Removed Field: "12"',
      'Removed field is rendered correctly'
    );

    await componentA11yAudit(this.element, assert);
  });

  test('job object diffs', async function (assert) {
    assert.expect(9);

    this.set('diff', {
      ID: 'test-case-2',
      Type: 'Edited',
      Objects: [
        {
          Name: 'ComplexProperty',
          Type: 'Edited',
          Objects: null,
          Fields: [
            field('Prop 1', 'added', 'prop-1-value'),
            field('Prop 2', 'none', 'prop-2-is-the-same'),
            field('Prop 3', 'edited', 'new value', 'some old value'),
            field('Prop 4', 'deleted', 'delete me'),
          ],
        },
        {
          Name: 'DeepConfiguration',
          Type: 'Added',
          Objects: [
            {
              Name: 'VP Props',
              Type: 'Added',
              Objects: null,
              Fields: [
                field('Engineering', 'added', 'Regina Phalange'),
                field('Customer Support', 'added', 'Jerome Hendricks'),
                field('HR', 'added', 'Jack Blue'),
                field('Sales', 'added', 'Maria Lopez'),
              ],
            },
          ],
          Fields: [field('Executive Prop', 'added', 'in charge')],
        },
        {
          Name: 'DatedStuff',
          Type: 'Deleted',
          Objects: null,
          Fields: [field('Deprecated', 'deleted', 'useless')],
        },
      ],
      Fields: null,
    });

    await render(commonTemplate);

    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="added"]'
        ).textContent
      ).startsWith('+ DeepConfiguration {'),
      'Added object starts with a JSON block'
    );
    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="edited"]'
        ).textContent
      ).startsWith('+/- ComplexProperty {'),
      'Edited object starts with a JSON block'
    );
    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="deleted"]'
        ).textContent
      ).startsWith('- DatedStuff {'),
      'Removed object starts with a JSON block'
    );

    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="added"]'
        ).textContent
      ).endsWith('}'),
      'Added object ends the JSON block'
    );
    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="edited"]'
        ).textContent
      ).endsWith('}'),
      'Edited object starts with a JSON block'
    );
    assert.ok(
      cleanWhitespace(
        find(
          '[data-test-diff-section-label="object"][data-test-diff-field="deleted"]'
        ).textContent
      ).endsWith('}'),
      'Removed object ends the JSON block'
    );

    assert.equal(
      findAll(
        '[data-test-diff-section-label="object"][data-test-diff-field="added"] > [data-test-diff-section-label]'
      ).length,
      this.diff.Objects[1].Objects.length + this.diff.Objects[1].Fields.length,
      'Edited block contains each nested field and object'
    );

    assert.equal(
      findAll(
        '[data-test-diff-section-label="object"][data-test-diff-field="added"] [data-test-diff-section-label="object"] [data-test-diff-section-label="field"]'
      ).length,
      this.diff.Objects[1].Objects[0].Fields.length,
      'Objects within objects are rendered'
    );

    await componentA11yAudit(this.element, assert);
  });

  function field(name, type, newVal, oldVal) {
    switch (type) {
      case 'added':
        return {
          Annotations: null,
          New: newVal,
          Old: '',
          Type: 'Added',
          Name: name,
        };
      case 'deleted':
        return {
          Annotations: null,
          New: '',
          Old: newVal,
          Type: 'Deleted',
          Name: name,
        };
      case 'edited':
        return {
          Annotations: null,
          New: newVal,
          Old: oldVal,
          Type: 'Edited',
          Name: name,
        };
    }
    return {
      Annotations: null,
      New: newVal,
      Old: oldVal,
      Type: 'None',
      Name: name,
    };
  }
});
