/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import sinon from 'sinon';
import RSVP from 'rsvp';
import { formatBytes } from 'nomad-ui/utils/units';

module('Integration | Component | image file', function (hooks) {
  setupRenderingTest(hooks);

  const commonTemplate = hbs`
    <ImageFile @src={{src}} @alt={{alt}} @size={{size}} />
  `;

  const commonProperties = {
    src: 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',
    alt: 'This is the alt text',
    size: 123456,
  };

  test('component displays the image', async function (assert) {
    assert.expect(3);

    this.setProperties(commonProperties);

    await render(commonTemplate);

    assert.ok(find('img'), 'Image is in the DOM');
    assert.equal(
      find('img').getAttribute('src'),
      commonProperties.src,
      `src is ${commonProperties.src}`
    );

    await componentA11yAudit(this.element, assert);
  });

  test('the image is wrapped in an anchor that links directly to the image', async function (assert) {
    this.setProperties(commonProperties);

    await render(commonTemplate);

    assert.ok(find('a'), 'Anchor');
    assert.ok(find('a > img'), 'Image in anchor');
    assert.equal(
      find('a').getAttribute('href'),
      commonProperties.src,
      `href is ${commonProperties.src}`
    );
    assert.equal(
      find('a').getAttribute('target'),
      '_blank',
      'Anchor opens to a new tab'
    );
    assert.equal(
      find('a').getAttribute('rel'),
      'noopener noreferrer',
      'Anchor rel correctly bars openers and referrers'
    );
  });

  test('component updates image meta when the image loads', async function (assert) {
    const { spy, wrapper, notifier } = notifyingSpy();

    this.setProperties(commonProperties);
    this.set('spy', wrapper);

    render(hbs`
      <ImageFile @src={{src}} @alt={{alt}} @size={{size}} @updateImageMeta={{spy}} />
    `);

    await notifier;
    assert.ok(spy.calledOnce);
  });

  test('component shows the width, height, and size of the image', async function (assert) {
    this.setProperties(commonProperties);

    await render(commonTemplate);

    const statsEl = find('[data-test-file-stats]');
    assert.ok(
      /\d+px \u00d7 \d+px/.test(statsEl.textContent),
      'Width and height are formatted correctly'
    );
    assert.ok(
      statsEl.textContent
        .trim()
        .endsWith(formatBytes(commonProperties.size) + ')'),
      'Human-formatted size is included'
    );
  });
});

function notifyingSpy() {
  // The notifier must resolve when the spy wrapper is called
  let dispatch;
  const notifier = new RSVP.Promise((resolve) => {
    dispatch = resolve;
  });

  const spy = sinon.spy();

  // The spy wrapper must call the spy, passing all arguments through, and it must
  // call dispatch in order to resolve the promise.
  const wrapper = (...args) => {
    spy(...args);
    dispatch();
  };

  // All three pieces are required to wire up a component, pause test execution, and
  // write assertions.
  return { spy, wrapper, notifier };
}
