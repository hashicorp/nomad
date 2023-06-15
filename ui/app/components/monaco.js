import Component from '@glimmer/component';
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import { action } from '@ember/object';

export default class extends Component {
  @action
  setupMonaco(element) {
    monaco.editor.create(element, {
      value: 'console.log("Hello, world")',
      language: 'javascript',
    });
  }
}
