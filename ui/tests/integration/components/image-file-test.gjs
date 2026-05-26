/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render, waitFor } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import sinon from 'sinon';
import RSVP from 'rsvp';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { formatBytes } from 'nomad-ui/utils/units';
import ImageFile from 'nomad-ui/components/image-file';

module('Integration | Component | image file', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = {
    src: 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',
    alt: 'This is the alt text',
    size: 123456,
  };

  test('component displays the image', async function (assert) {
    const { src, alt, size } = commonProperties;

    await render(
      <template>
        <ImageFile @src={{src}} @alt={{alt}} @size={{size}} />
      </template>,
    );

    assert.ok(find('img'), 'Image is in the DOM');
    assert.deepEqual(find('img').getAttribute('src'), src, `src is ${src}`);

    await componentA11yAudit(find('[data-test-image-file]'), assert);
  });

  test('the image is wrapped in an anchor that links directly to the image', async function (assert) {
    const { src, alt, size } = commonProperties;

    await render(
      <template>
        <ImageFile @src={{src}} @alt={{alt}} @size={{size}} />
      </template>,
    );

    assert.ok(find('a'), 'Anchor');
    assert.ok(find('a > img'), 'Image in anchor');
    assert.deepEqual(find('a').getAttribute('href'), src, `href is ${src}`);
    assert.deepEqual(
      find('a').getAttribute('target'),
      '_blank',
      'Anchor opens to a new tab',
    );
    assert.deepEqual(
      find('a').getAttribute('rel'),
      'noopener noreferrer',
      'Anchor rel correctly bars openers and referrers',
    );
  });

  test('component updates image meta when the image loads', async function (assert) {
    const { spy, wrapper, notifier } = notifyingSpy();
    const { src, alt, size } = commonProperties;

    await render(
      <template>
        <ImageFile
          @src={{src}}
          @alt={{alt}}
          @size={{size}}
          @updateImageMeta={{wrapper}}
        />
      </template>,
    );

    await notifier;
    assert.ok(spy.calledOnce);
  });

  test('component shows the width, height, and size of the image', async function (assert) {
    const { src, alt, size } = commonProperties;

    await render(
      <template>
        <ImageFile @src={{src}} @alt={{alt}} @size={{size}} />
      </template>,
    );

    await waitFor('[data-test-file-stats]');
    const statsEl = find('[data-test-file-stats]');

    assert.ok(
      /\d+px\s*\u00d7\s*\d+px/.test(statsEl.textContent),
      'Width and height are formatted correctly',
    );
    assert.ok(
      statsEl.textContent.trim().endsWith(formatBytes(size) + ')'),
      'Human-formatted size is included',
    );
  });
});

function notifyingSpy() {
  // The notifier must resolve when the spy wrapper is called.
  let dispatch;
  const notifier = new RSVP.Promise((resolve) => {
    dispatch = resolve;
  });

  const spy = sinon.spy();

  // The spy wrapper calls through and resolves the notifier.
  const wrapper = (...args) => {
    spy(...args);
    dispatch();
  };

  return { spy, wrapper, notifier };
}
