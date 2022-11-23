import Component from '@ember/component';
import { computed } from '@ember/object';
import {
  classNames,
  attributeBindings,
  classNameBindings,
  tagName,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
// import localStorageProperty from 'nomad-ui/utils/properties/local-storage';

@classic
@tagName('th')
@attributeBindings('title')
@classNames('is-selectable')
@classNameBindings('isActive:is-active', 'sortDescending:desc:asc')
export default class SortBy extends Component {

  didInsertElement() {
    super.didInsertElement(...arguments);
    if (this.resizable) {
      console.log('better check localstorage for', this.element.innerText);
      const width = window.localStorage.getItem(`nomad.column.width.${this.element.innerText}`);
      console.log('width set?', width);
       if (width) {
        this.element.style.width = width;
       }
    }
  }

  // The prop that the table is currently sorted by
  currentProp = '';

  // The prop this sorter controls
  prop = '';

  @computed('currentProp', 'prop')
  get isActive() {
    return this.currentProp === this.prop;
  }

  @computed('sortDescending', 'isActive')
  get shouldSortDescending() {
    return !this.isActive || !this.sortDescending;
  }

  get canResize() {
    return this.resizable;
  }

  resizing = false;
  startingWidth = 0;
  startingPosition = 0;
  td = null;

  @action
  startResize(e) {
    console.log('start resize', e.target);
    document.addEventListener('mousemove', this.resize);
    document.addEventListener('mouseup', this.endResize);
    this.resizing = true;
    this.td = e.target.parentElement;
    console.log('elly', this.td);
    this.startingWidth = this.td.clientWidth;
    this.startingPosition = e.clientX;
  }
  @action
  resize(e) {
    if (this.resizing) {
      e.preventDefault();
      // console.log('resizing', e.offsetX, e.clientX);
      const newWidth = this.startingWidth + (e.clientX - this.startingPosition);
      // console.log('NEW WIDTH IS', newWidth);
      this.td.style.width = `${newWidth}px`;
    }
  }
  @action
  endResize(e) {
    if (this.resizing) {
      console.log('end resize', e);
      // document.removeEventListener('mouseup', this.endResize);
      document.removeEventListener('mousemove', this.resize);
      console.log('STARTING WIDTH WAS', this.startingWidth, 'FINAL WIDTH IS', this.td.style.width);
      window.localStorage.setItem(
        `nomad.column.width.${this.td.innerText}`,
        this.td.style.width
      );
      this.resizing = false;
      this.startingWidth = 0;
      this.startingPosition = 0;
      this.td = null;
    }
  }

  // @action resetSize(e) {
  //   console.log('reset size', e);
  //   window.localStorage.removeItem(`nomad.column.width.${this.element.innerText}`);
  //   this.element.style.width = '';
  // }
}
