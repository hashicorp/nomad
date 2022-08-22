import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { inject as service } from '@ember/service';

@tagName('')
export default class ClientSubnav extends Component {
  @service keyboard;
}
