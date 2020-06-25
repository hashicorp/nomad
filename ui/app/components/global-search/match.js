import Component from '@ember/component';
import { tagName } from '@ember-decorators/component';
import { computed, get } from '@ember/object';
import { alias } from '@ember/object/computed';

@tagName('')
export default class GlobalSearchMatch extends Component {
  @alias('match.fuzzySearchMatches.firstObject') firstMatch;

  @computed('match.name')
  get label() {
    return get(this, 'match.name') || '';
  }

  @computed('label', 'firstMatch.indices.[]')
  get substrings() {
    const indices = get(this, 'firstMatch.indices');
    const labelLength = this.label.length;

    if (indices) {
      return indices.reduce((substrings, [startIndex, endIndex], indicesIndex) => {
        if (indicesIndex === 0 && startIndex > 0) {
          substrings.push({
            isHighlighted: false,
            string: this.label.substring(0, startIndex)
          });
        }

        substrings.push({
          isHighlighted: true,
          string: this.label.substring(startIndex, endIndex + 1)
        });

        let endIndexOfNextUnhighlightedSubstring;

        if (indicesIndex === indices.length - 1) {
          endIndexOfNextUnhighlightedSubstring = labelLength;
        } else {
          const nextIndices = indices[indicesIndex + 1];
          endIndexOfNextUnhighlightedSubstring = nextIndices[0];
        }

        substrings.push({
          isHighlighted: false,
          string: this.label.substring(endIndex + 1, endIndexOfNextUnhighlightedSubstring)
        });

        return substrings;
      }, []);
    } else {
      return null;
    }
  }
}
