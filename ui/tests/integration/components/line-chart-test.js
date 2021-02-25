import { findAll, click, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';
import moment from 'moment';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const REF_DATE = new Date();

module('Integration | Component | line-chart', function(hooks) {
  setupRenderingTest(hooks);

  test('when a chart has annotations, they are rendered in order', async function(assert) {
    const annotations = [
      { x: 2, type: 'info' },
      { x: 1, type: 'error' },
      { x: 3, type: 'info' },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
    });

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} />
        </:after>
      </LineChart>
    `);

    const sortedAnnotations = annotations.sortBy('x');
    findAll('[data-test-annotation]').forEach((annotation, idx) => {
      const datum = sortedAnnotations[idx];
      assert.equal(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${datum.x}`
      );
    });

    await componentA11yAudit(this.element, assert);
  });

  test('when a chart has annotations and is timeseries, annotations are sorted reverse-chronologically', async function(assert) {
    const annotations = [
      {
        x: moment(REF_DATE)
          .add(2, 'd')
          .toDate(),
        type: 'info',
      },
      {
        x: moment(REF_DATE)
          .add(1, 'd')
          .toDate(),
        type: 'error',
      },
      {
        x: moment(REF_DATE)
          .add(3, 'd')
          .toDate(),
        type: 'info',
      },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
    });

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @timeseries={{true}}
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} />
        </:after>
      </LineChart>
    `);

    const sortedAnnotations = annotations.sortBy('x').reverse();
    findAll('[data-test-annotation]').forEach((annotation, idx) => {
      const datum = sortedAnnotations[idx];
      assert.equal(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${moment(datum.x).format('MMM DD, HH:mm')}`
      );
    });
  });

  test('clicking annotations calls the onAnnotationClick action with the annotation as an argument', async function(assert) {
    const annotations = [{ x: 2, type: 'info', meta: { data: 'here' } }];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
      click: sinon.spy(),
    });

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} @annotationClick={{this.click}} />
        </:after>
      </LineChart>
    `);

    await click('[data-test-annotation] button');
    assert.ok(this.click.calledWith(annotations[0]));
  });

  test('annotations will have staggered heights when too close to be positioned side-by-side', async function(assert) {
    const annotations = [
      { x: 2, type: 'info' },
      { x: 2.4, type: 'error' },
      { x: 9, type: 'info' },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
      click: sinon.spy(),
    });

    await render(hbs`
      <div style="width:200px;">
        <LineChart
          @xProp="x"
          @yProp="y"
          @data={{this.data}}>
          <:after as |c|>
            <c.VAnnotations @annotations={{this.annotations}} />
          </:after>
        </LineChart>
      </div>
    `);

    const annotationEls = findAll('[data-test-annotation]');
    assert.notOk(annotationEls[0].classList.contains('is-staggered'));
    assert.ok(annotationEls[1].classList.contains('is-staggered'));
    assert.notOk(annotationEls[2].classList.contains('is-staggered'));

    await componentA11yAudit(this.element, assert);
  });
});
