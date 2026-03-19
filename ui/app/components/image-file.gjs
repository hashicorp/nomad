/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { on } from '@ember/modifier';
import formatBytes from 'nomad-ui/helpers/format-bytes';

export default class ImageFile extends Component {
  @tracked width = 0;
  @tracked height = 0;

  get fileName() {
    const src = this.args.src;
    if (!src) return undefined;
    return src.includes('/') ? src.match(/^.*\/(.*)$/)[1] : src;
  }

  get altText() {
    return this.args.alt || this.fileName;
  }

  get hasDimensions() {
    return this.width && this.height;
  }

  handleImageLoad = (event) => {
    const img = event.target;
    this.width = img.naturalWidth;
    this.height = img.naturalHeight;

    if (typeof this.args.updateImageMeta === 'function') {
      this.args.updateImageMeta(event);
    }
  };

  <template>
    <figure class="image-file" data-test-image-file ...attributes>
      <a
        data-test-image-link
        href={{@src}}
        target="_blank"
        rel="noopener noreferrer"
        class="image-file-image"
      >
        <img
          data-test-image
          src={{@src}}
          alt={{this.altText}}
          title={{this.fileName}}
          {{on "load" this.handleImageLoad}}
        />
      </a>
      <figcaption class="image-file-caption">
        <span class="image-file-caption-primary">
          <strong data-test-file-name>{{this.fileName}}</strong>
          {{#if this.hasDimensions}}
            <span data-test-file-stats>({{this.width}}px &times;
              {{this.height}}px{{#if @size}},
                {{formatBytes @size}}{{/if}})</span>
          {{/if}}
        </span>
      </figcaption>
    </figure>
  </template>
}
