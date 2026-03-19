/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { htmlSafe } from '@ember/template';
import { click, find, findAll, render, settled } from '@ember/test-helpers';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import FlexMasonry from 'nomad-ui/components/flex-masonry';

// Used to prevent XSS warnings in console
const h = (height) => htmlSafe(`height:${height}px`);

module('Integration | Component | FlexMasonry', function (hooks) {
  setupRenderingTest(hooks);

  test('presents as a single div when @items is empty', async function (assert) {
    const items = [];
    const columns = undefined;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} />
      </template>,
    );

    const containerEl = find('[data-test-flex-masonry]');
    assert.ok(containerEl);
    assert.deepEqual(containerEl.tagName.toLowerCase(), 'div');
    assert.deepEqual(containerEl.children.length, 0);

    await componentA11yAudit(containerEl, assert);
  });

  test('each item in @items gets wrapped in a flex-masonry-item wrapper', async function (assert) {
    const items = ['one', 'two', 'three'];
    const columns = 2;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} as |item|>
          <p>{{item}}</p>
        </FlexMasonry>
      </template>,
    );

    assert.deepEqual(
      findAll('[data-test-flex-masonry-item]').length,
      items.length,
    );
  });

  test('the @withSpacing arg adds the with-spacing class', async function (assert) {
    const items = [];
    const columns = 1;

    await render(
      <template>
        <FlexMasonry
          @items={{items}}
          @columns={{columns}}
          @withSpacing={{true}}
        />
      </template>,
    );

    assert.ok(
      find('[data-test-flex-masonry]').classList.contains('with-spacing'),
    );
  });

  test('individual items along with the reflow action are yielded', async function (assert) {
    class State {
      @tracked height = h(50);
    }

    const state = new State();
    const items = ['one', 'two'];
    const columns = 2;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} as |item reflow|>
          <div style={{state.height}} {{on "click" reflow}}>{{item}}</div>
        </FlexMasonry>
      </template>,
    );

    const containerEl = find('[data-test-flex-masonry]');
    assert.deepEqual(containerEl.style.maxHeight, '51px');
    assert.ok(containerEl.textContent.includes('one'));
    assert.ok(containerEl.textContent.includes('two'));

    state.height = h(500);
    await settled();
    assert.deepEqual(containerEl.style.maxHeight, '51px');

    // The height of the div changes when reflow is called
    await click('[data-test-flex-masonry-item]:first-child div');

    assert.deepEqual(containerEl.style.maxHeight, '501px');
  });

  test('items are rendered to the DOM in the order they were passed into the component', async function (assert) {
    const items = [
      { text: 'One', height: h(20) },
      { text: 'Two', height: h(100) },
      { text: 'Three', height: h(20) },
      { text: 'Four', height: h(20) },
    ];
    const columns = 2;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} as |item|>
          <div style={{item.height}}>{{item.text}}</div>
        </FlexMasonry>
      </template>,
    );

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      assert.deepEqual(el.textContent.trim(), items[index].text);
    });
  });

  test('each item gets an order property', async function (assert) {
    const items = [
      { text: 'One', height: h(20), expectedOrder: 0 },
      { text: 'Two', height: h(100), expectedOrder: 3 },
      { text: 'Three', height: h(20), expectedOrder: 1 },
      { text: 'Four', height: h(20), expectedOrder: 2 },
    ];
    const columns = 2;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} as |item|>
          <div style={{item.height}}>{{item.text}}</div>
        </FlexMasonry>
      </template>,
    );

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      assert.strictEqual(Number(el.style.order), items[index].expectedOrder);
    });
  });

  test('the last item in each column gets a specific flex-basis value', async function (assert) {
    const items = [
      { text: 'One', height: h(20) },
      { text: 'Two', height: h(100), flexBasis: '100px' },
      { text: 'Three', height: h(20) },
      { text: 'Four', height: h(100), flexBasis: '100px' },
      { text: 'Five', height: h(20), flexBasis: '80px' },
      { text: 'Six', height: h(20), flexBasis: '80px' },
    ];
    const columns = 4;

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{columns}} as |item|>
          <div style={{item.height}}>{{item.text}}</div>
        </FlexMasonry>
      </template>,
    );

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      if (el.style.flexBasis) {
        assert.deepEqual(el.style.flexBasis, items[index].flexBasis);
      }
    });
  });

  test('when a multi-column layout becomes a single column layout, all inline-styles are reset', async function (assert) {
    class State {
      @tracked columns = 4;
    }

    const state = new State();
    const items = [
      { text: 'One', height: h(20) },
      { text: 'Two', height: h(100) },
      { text: 'Three', height: h(20) },
      { text: 'Four', height: h(100) },
      { text: 'Five', height: h(20) },
      { text: 'Six', height: h(20) },
    ];

    await render(
      <template>
        <FlexMasonry @items={{items}} @columns={{state.columns}} as |item|>
          <div style={{item.height}}>{{item.text}}</div>
        </FlexMasonry>
      </template>,
    );

    assert.deepEqual(find('[data-test-flex-masonry]').style.maxHeight, '101px');

    state.columns = 1;
    await settled();

    findAll('[data-test-flex-masonry-item]').forEach((el) => {
      assert.deepEqual(el.style.flexBasis, '');
      assert.deepEqual(el.style.order, '');
    });

    assert.deepEqual(find('[data-test-flex-masonry]').style.maxHeight, '');
  });
});
