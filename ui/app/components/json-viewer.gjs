/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import codeMirror from 'nomad-ui/modifiers/code-mirror';
import stringifyObject from 'nomad-ui/helpers/stringify-object';

export default class JsonViewer extends Component {
  get rootClass() {
    return this.args.fluidHeight
      ? 'json-viewer has-fluid-height'
      : 'json-viewer';
  }

  <template>
    <div class={{this.rootClass}} ...attributes>
      <div
        data-test-json-viewer
        {{codeMirror
          content=(stringifyObject @json)
          theme="hashi-read-only"
          readOnly=true
          screenReaderLabel="JSON Viewer"
        }}
      />
    </div>
  </template>
}
