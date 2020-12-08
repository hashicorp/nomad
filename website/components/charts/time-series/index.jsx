import React, { useState, useEffect } from 'react'
import { csv } from 'd3-fetch'
import { extent } from 'd3-array'
import { scaleTime, scaleLinear } from '@visx/scale'
import { Group } from '@visx/group'
import { GridColumns, GridRows } from '@visx/grid'
import { AxisBottom, AxisLeft } from '@visx/axis'
import { LinePath } from '@visx/shape'
import { curveBasis } from '@visx/curve'

export default function TimeSeriesChart({
  csvPath,
  xAxisColumn,
  lineColumns = [],
}) {
  const [data, setData] = useState([])
  useEffect(() => {
    csv(csvPath).then((d) => console.log(d) || setData(d))
  }, [])

  data.forEach((d) => {
    d[xAxisColumn] = new Date(d[xAxisColumn])
  })

  const values = lineColumns.map((c) => data.map((d) => +d[c])).flat()
  const colors = ['#00bc7f', '#b5b8c3', '#b5b8c3']

  const yScale = scaleLinear({
    domain: extent(values),
    nice: true,
  })

  const timeScale = scaleTime({
    domain: extent(data.map((d) => d[xAxisColumn])),
  })

  const width = 800
  const height = 500
  const margin = { top: 40, right: 30, bottom: 50, left: 40 }
  const yMax = height - margin.top - margin.bottom
  const xMax = width - margin.left - margin.right

  yScale.range([yMax, 0])
  timeScale.range([0, xMax])

  return (
    <div>
      <svg width={width} height={height}>
        <Group left={margin.left} top={margin.top}>
          <GridRows
            scale={yScale}
            width={xMax}
            height={yMax}
            stroke="#eeeeee"
          />
          <GridColumns
            scale={timeScale}
            width={xMax}
            height={yMax}
            stroke="#eeeeee"
          />
          <AxisBottom top={yMax} scale={timeScale} numTicks={5} />
          <AxisLeft scale={yScale} />
          {lineColumns
            .slice()
            .reverse()
            .map((col, i) => (
              <LinePath
                key={col}
                curve={curveBasis}
                data={data}
                x={(d) => timeScale(d[xAxisColumn])}
                y={(d) => yScale(+d[col])}
                stroke={colors[lineColumns.length - i - 1]}
                strokeWidth={2}
              />
            ))}
        </Group>
      </svg>
    </div>
  )
}
