/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import momentFrom from 'ember-moment/helpers/moment-from';
import formatBytes from 'nomad-ui/helpers/format-bytes';
import formatTs from 'nomad-ui/helpers/format-ts';
import FsLink from 'nomad-ui/components/fs/link';

export default class DirectoryEntry extends Component {
  get pathToEntry() {
    const path = this.args.path ?? '';
    const pathWithNoLeadingSlash = path.replace(/^\//, '');
    const name = encodeURIComponent(this.args.entry.Name);

    if (!pathWithNoLeadingSlash) {
      return name;
    }

    return `${pathWithNoLeadingSlash}/${name}`;
  }

  <template>
    <tr data-test-entry>
      <td>
        <FsLink
          @allocation={{@allocation}}
          @taskState={{@taskState}}
          @path={{this.pathToEntry}}
        >
          {{#if @entry.IsDir}}
            <HdsIcon @name="folder" @isInline={{true}} />
          {{else}}
            <HdsIcon @name="file" @isInline={{true}} />
          {{/if}}

          <span class="name" data-test-name>{{@entry.Name}}</span>
        </FsLink>
      </td>
      <td class="has-text-right" data-test-size>
        {{#unless @entry.IsDir}}{{formatBytes @entry.Size}}{{/unless}}
      </td>
      <td
        class="has-text-right"
        title={{formatTs @entry.ModTime}}
        data-test-last-modified
      >{{momentFrom @entry.ModTime interval=1000}}</td>
    </tr>
  </template>
}
