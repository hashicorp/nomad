import Component from '@ember/component';
import { computed } from '@ember/object';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('json-viewer')
export default class JsonViewer extends Component {
  json = null;

  @computed('json')
  get jsonStr() {
    return JSON.stringify(this.json, null, 2);
  }
}
