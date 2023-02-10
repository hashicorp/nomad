// @ts-check
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class JobPagePartsJobStatusPanelComponent extends Component {
  /**
   * @type {('current'|'historical')}
   */
  @tracked mode = 'current'; // can be either "current" or "historical"
}
