/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { htmlSafe } from '@ember/template';
import { click, find, findAll, render, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

// Used to prevent XSS warnings in console
const h = (height) => htmlSafe(`height:${height}px`);

module('Integration | Component | FlexMasonry', function (hooks) {
  setupRenderingTest(hooks);

  test('presents as a single div when @items is empty', async function (assert) {
    assert.expect(4);

    this.setProperties({
      items: [],
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}}>
      </FlexMasonry>
    `);

    const div = find('[data-test-flex-masonry]');
    assert.ok(div);
    assert.equal(div.tagName.toLowerCase(), 'div');
    assert.equal(div.children.length, 0);

    await componentA11yAudit(this.element, assert);
  });

  test('each item in @items gets wrapped in a flex-masonry-item wrapper', async function (assert) {
    this.setProperties({
      items: ['one', 'two', 'three'],
      columns: 2,
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item|>
        <p>{{item}}</p>
      </FlexMasonry>
    `);

    assert.equal(
      findAll('[data-test-flex-masonry-item]').length,
      this.items.length
    );
  });

  test('the @withSpacing arg adds the with-spacing class', async function (assert) {
    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}}
        @withSpacing={{true}}>
      </FlexMasonry>
    `);

    assert.ok(
      find('[data-test-flex-masonry]').classList.contains('with-spacing')
    );
  });

  test('individual items along with the reflow action are yielded', async function (assert) {
    this.setProperties({
      items: ['one', 'two'],
      columns: 2,
      height: h(50),
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item reflow|>
        <div style={{this.height}} {{on "click" reflow}}>{{item}}</div>
      </FlexMasonry>
    `);

    const div = find('[data-test-flex-masonry]');
    assert.equal(div.style.maxHeight, '51px');
    assert.ok(div.textContent.includes('one'));
    assert.ok(div.textContent.includes('two'));

    this.set('height', h(500));
    await settled();
    assert.equal(div.style.maxHeight, '51px');

    // The height of the div changes when reflow is called
    await click('[data-test-flex-masonry-item]:first-child div');

    assert.equal(div.style.maxHeight, '501px');
  });

  test('items are rendered to the DOM in the order they were passed into the component', async function (assert) {
    assert.expect(4);

    this.setProperties({
      items: [
        { text: 'One', height: h(20) },
        { text: 'Two', height: h(100) },
        { text: 'Three', height: h(20) },
        { text: 'Four', height: h(20) },
      ],
      columns: 2,
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item|>
        <div style={{item.height}}>{{item.text}}</div>
      </FlexMasonry>
    `);

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      assert.equal(el.textContent.trim(), this.items[index].text);
    });
  });

  test('each item gets an order property', async function (assert) {
    assert.expect(4);

    this.setProperties({
      items: [
        { text: 'One', height: h(20), expectedOrder: 0 },
        { text: 'Two', height: h(100), expectedOrder: 3 },
        { text: 'Three', height: h(20), expectedOrder: 1 },
        { text: 'Four', height: h(20), expectedOrder: 2 },
      ],
      columns: 2,
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item|>
        <div style={{item.height}}>{{item.text}}</div>
      </FlexMasonry>
    `);

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      assert.equal(el.style.order, this.items[index].expectedOrder);
    });
  });

  test('the last item in each column gets a specific flex-basis value', async function (assert) {
    assert.expect(4);

    this.setProperties({
      items: [
        { text: 'One', height: h(20) },
        { text: 'Two', height: h(100), flexBasis: '100px' },
        { text: 'Three', height: h(20) },
        { text: 'Four', height: h(100), flexBasis: '100px' },
        { text: 'Five', height: h(20), flexBasis: '80px' },
        { text: 'Six', height: h(20), flexBasis: '80px' },
      ],
      columns: 4,
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item|>
        <div style={{item.height}}>{{item.text}}</div>
      </FlexMasonry>
    `);

    findAll('[data-test-flex-masonry-item]').forEach((el, index) => {
      if (el.style.flexBasis) {
        /* eslint-disable-next-line qunit/no-conditional-assertions */
        assert.equal(el.style.flexBasis, this.items[index].flexBasis);
      }
    });
  });

  test('when a multi-column layout becomes a single column layout, all inline-styles are reset', async function (assert) {
    assert.expect(14);

    this.setProperties({
      items: [
        { text: 'One', height: h(20) },
        { text: 'Two', height: h(100) },
        { text: 'Three', height: h(20) },
        { text: 'Four', height: h(100) },
        { text: 'Five', height: h(20) },
        { text: 'Six', height: h(20) },
      ],
      columns: 4,
    });

    await render(hbs`
      <FlexMasonry
        @items={{this.items}}
        @columns={{this.columns}} as |item|>
        <div style={{item.height}}>{{item.text}}</div>
      </FlexMasonry>
    `);

    assert.equal(find('[data-test-flex-masonry]').style.maxHeight, '101px');

    this.set('columns', 1);
    await settled();

    findAll('[data-test-flex-masonry-item]').forEach((el) => {
      assert.equal(el.style.flexBasis, '');
      assert.equal(el.style.order, '');
    });

    assert.equal(find('[data-test-flex-masonry]').style.maxHeight, '');
  });
});
