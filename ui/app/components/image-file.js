/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('figure')
@classNames('image-file')
@attributeBindings('data-test-image-file')
export default class ImageFile extends Component {
  'data-test-image-file' = true;

  src = null;
  alt = null;
  size = null;

  // Set by updateImageMeta
  width = 0;
  height = 0;

  @computed('src')
  get fileName() {
    if (!this.src) return undefined;
    return this.src.includes('/') ? this.src.match(/^.*\/(.*)$/)[1] : this.src;
  }

  updateImageMeta(event) {
    const img = event.target;
    this.setProperties({
      width: img.naturalWidth,
      height: img.naturalHeight,
    });
  }
}
