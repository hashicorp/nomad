import { Fragment } from 'react'

export default function PlacementTable({ groups = [] }) {
  return (
    <table className="g-placement-table">
      <thead>
        <tr>
          <td width="120" className="head">
            Placement
          </td>
          <td>
            {Array.isArray(groups[0]) ? (
              groups.map(subgroup => {
                return (
                  <Fragment key={subgroup.join('')}>
                    <code
                      dangerouslySetInnerHTML={{
                        __html: wrapLastItem(subgroup, 'strong').join(' -> ')
                      }}
                    />
                    <br />
                  </Fragment>
                )
              })
            ) : (
              <code
                dangerouslySetInnerHTML={{
                  __html: wrapLastItem(groups, 'strong').join(' -> ')
                }}
              />
            )}
          </td>
        </tr>
      </thead>
    </table>
  )
}

function wrapLastItem(arr, wrapper) {
  arr[arr.length - 1] = `<${wrapper}>${arr[arr.length - 1]}</${wrapper}>`
  return arr
}
