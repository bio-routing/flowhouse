var filtersCount = 0;

$(document).ready(function() {
  var start = formatTimestamp(new Date(((new Date() / 1000) - 900 - new Date().getTimezoneOffset() * 60)* 1000));
  if ($("#time_start").val() == "") {
    $("#time_start").val(start);
  }

  var end = formatTimestamp(new Date(((new Date() / 1000) - new Date().getTimezoneOffset() * 60)* 1000));
  if ($("#time_end").val() == "") {
    $("#time_end").val(end);
  }

  $("#filterPlus").click(addFilter);
  $("form").on('submit', submitQuery);

  google.charts.load('current', {
   'packages': ['corechart']
  });
   
  window.onhashchange = function () {
    google.charts.setOnLoadCallback(drawChart);
  }

  google.charts.setOnLoadCallback(drawChart);

  populateFields();
});

function addFilter() {
  const filterTemplate = $("#filterTemplate").html().replace(/__NUM__/g, filtersCount);
  $("#filters").append(filterTemplate);

  const $filterField = $(`#filter_field\\[${filtersCount}\\]`);
  const $filterValue = $(`#filter_value\\[${filtersCount}\\]`);
  const $filterRemove = $(`#filter_remove\\[${filtersCount}\\]`);

  $filterField.change(function() {
    const fieldName = $(this).val();
    const filterNum = $(this).attr("id").match(/\d+/)[0];
    $filterValue.attr("name", fieldName);
    loadValues(filterNum, fieldName);
  });

  $filterRemove.click(function() {
    $(this).closest('.row').remove();
  });

  filtersCount++;
}


function parseParams(str) {
  return str.split('&').reduce(function(params, param) {
    const [key, value] = param.split('=').map(decodeURIComponent);
    params[key] = value.replace(/\+/g, ' ');
    return params;
  }, {});
}

function populateFields() {
  var query = location.href.split("#")[1];
  if (!query) {
    return;
  }

  var queryEquations = query.split('&');
  for (var i = 0; i < queryEquations.length; i++) {
    var e = queryEquations[i].split('=');
    var k = e[0];
    var v = decodeURIComponent(e[1]);

    if (k == "breakdown") {
      $("#breakdown option[value=" + v + "]").attr('selected', 'selected');
      continue;
    }
    
    if (k == "time_start") {
      $("#time_start").val(v);
      continue;
    }
    
    if (k == "time_end") {
      $("#time_end").val(v);
      continue;
    }

    if (k == "topFlows") {
      $("#topFlows").val(v);
      continue;
    }

    if (k.match(/^filter_field/)) {
      continue;
    }

    var fieldIndex = addFilter();
    $("#filter_field\\[" + fieldIndex + "\\]").val(k);
    $("#filter_field\\[" + fieldIndex + "\\]").trigger("change");
    $("#filter_value\\[" + fieldIndex + "\\]").val(v);
  }
}

function submitQuery() {
  event.preventDefault();

  // Validate 'topFlows' box
  const topFlows = $('#topFlows').val();
  const topFlowsInt = parseInt(topFlows, 10);
  if (isNaN(topFlowsInt) || topFlowsInt < 1 || topFlowsInt > 10000) {
    alert("Incorrect 'Top Flows': please enter a valid integer between 1 and 10000.");
    return false;
  }

  params = $('form').serialize();
  params += '&topFlows=' + encodeURIComponent(topFlows);
  location.href = "#" + params
  return false
}

function drawChart() {
  var query = location.href.split("#")[1]
  if (!query) {
    return;
  }

  $.ajax({
    type: "GET",
    url: "/query?" + query,
    dataType: "text",
    success: function(rdata, status, xhr) {
      if (rdata == undefined) {
        $("#chart_div").text("No data found")
          return
        }
      renderChart(rdata)
    },
    error: function(xhr) {
      $("#chart_div").text(xhr.responseText)
    }
  })
}

function renderChart(rdata) {
  pres = Papa.parse(rdata.trim());

  var filtered = [pres.data[0]];
  for (const row of pres.data) {
    const hasNonZero = row.slice(1).some(val => {
      const num = parseFloat((val || '').trim());
      return !isNaN(num) && num !== 0;
    });
    if (hasNonZero) {
      filtered.push(row);
    }
  }

  var data = [];
  for (var i = 0; i < filtered.length; i++) {
    data[i] = [];
    for (var j = 0; j < filtered[i].length; j++) {
      var x = filtered[i][j];
      if (i !== 0 && j !== 0) {
        x = parseFloat((x || '').trim());
        if (isNaN(x)) x = 0;
      }
      data[i][j] = x;
    }
  }

  if (!window.seriesVisibility || window.seriesVisibility.length !== data[0].length - 1) {
    window.seriesVisibility = Array(data[0].length - 1).fill(true);
  }

  var filteredData = [];
  for (var i = 0; i < data.length; i++) {
    var row = [data[i][0]];
    for (var j = 1; j < data[i].length; j++) {
      if (window.seriesVisibility[j - 1]) {
        row.push(data[i][j]);
      }
    }
    filteredData.push(row);
  }

  if (filteredData[0].length < 2) {
    $("#chart_div").text("No series selected.");
    document.getElementById('custom_legend').innerHTML = '';
    return;
  }

  var chartData = google.visualization.arrayToDataTable(filteredData);

  var options = {
    isStacked: false,
    title: 'Flow Mbps',
    titleTextStyle: {
      fontSize: 24,
      bold: true,
      color: '#333'
    },
    hAxis: {
      title: 'Time',
      slantedText: true,
      slantedTextAngle: 60,
      showTextEvery: 10,
      titleTextStyle: {
        color: '#333',
        italic: false,
        bold: true,
        fontSize: 18
      },
      gridlines: {
        color: '#f3f3f3',
        count: 10
      },
      minorGridlines: {
        color: '#e9e9e9'
      },
      textStyle: {
        color: '#333',
        fontSize: 12
      }
    },
    vAxis: {
      minValue: 0,
      title: 'Megabits per second',
      titleTextStyle: {
        color: '#333',
        italic: false,
        bold: true,
        fontSize: 18
      },
      gridlines: {
        color: '#f3f3f3',
        count: 10
      },
      minorGridlines: {
        color: '#e9e9e9'
      },
      textStyle: {
        color: '#333',
        fontSize: 12
      }
    },
    height: screen.height * 0.7,
    chartArea: {
      width: '90%', 
      height: '70%',
      top: '5%',
      backgroundColor: {
        stroke: '#ccc',
        strokeWidth: 1
      }
    },
    backgroundColor: '#ffffff',
    colors: ['#2196F3', '#4CAF50', '#FFC107', '#FF5722', '#9C27B0'],
    animation: {
      startup: true,
      duration: 1000,
      easing: 'out'
    },
    legend: {
      position: 'none'
    },
    tooltip: {
      textStyle: {
        color: '#333',
        fontSize: 12
      },
      showColorCode: true
    },
    lineWidth: 2,
    pointSize: 1,
    series: {
      0: { lineDashStyle: [4, 4] },
      1: { lineDashStyle: [2, 2] },
      2: { lineDashStyle: [4, 2] },
      3: { lineDashStyle: [2, 4] },
      4: { lineDashStyle: [1, 1] }
    }
  };

  var chart = new google.visualization.AreaChart(document.getElementById('chart_div'));
  chart.draw(chartData, options);

  renderLegendTable();

  function renderLegendTable() {
    const flowStats = [];
    for (let i = 1; i < data[0].length; i++) {
      let max = -Infinity;
      for (let j = 1; j < data.length; j++) {
        const val = data[j][i];
        if (typeof val === "number" && !isNaN(val)) {
          if (val > max) max = val;
        }
      }
      flowStats.push({
        index: i,
        label: data[0][i],
        max: max === -Infinity ? 0 : max
      });
    }

    // Sorting logic
    if (!window.legendSort) window.legendSort = { key: "label", asc: true };
    const sortKey = window.legendSort.key;
    const sortAsc = window.legendSort.asc;

    flowStats.sort((a, b) => {
      switch (sortKey) {
        case "label":
          return sortAsc
            ? a.label.localeCompare(b.label)
            : b.label.localeCompare(a.label);
        case "max":
          return sortAsc
            ? a.max - b.max
            : b.max - a.max;
        default:
          return 0;
      }
    });

    const customLegendDiv = document.getElementById('custom_legend');
    customLegendDiv.innerHTML = `
    <div class="legend-help-tip">
      <strong>Usage:</strong><br>
      <span style="color:#2196F3;font-weight:bold;">• Click a flow</span> to show only that flow. Click again to show all.<br>
      <span style="color:#4CAF50;font-weight:bold;">• Ctrl/Cmd/Option + Click</span> to add or remove flows.<br>
      <span style="color:#FF5722;font-weight:bold;">• Click a column header</span> to sort the legend.
    </div>
    `;

    const colors = options.colors;
    const table = document.createElement('table');
    table.classList.add('table', 'table-sm', 'table-bordered');
    const thead = document.createElement('thead');
    const headRow = document.createElement('tr');

    function makeHeaderCell(text, key) {
      const th = document.createElement('th');
      th.textContent = text;
      th.style.cursor = 'pointer';
      th.style.userSelect = 'none';
      th.addEventListener('click', (e) => {
        if (window.legendSort.key === key) {
          window.legendSort.asc = !window.legendSort.asc;
        } else {
          window.legendSort.key = key;
          // Default: sort by MAX Mbps descending, FLOW ascending
          window.legendSort.asc = (key === 'label');
        }
        renderLegendTable();
        e.stopPropagation();
      });
      if (window.legendSort.key === key) {
        th.textContent += window.legendSort.asc ? ' ▲' : ' ▼';
      }
      return th;
    }

    headRow.appendChild(document.createElement('th')); // color cell (empty)
    headRow.appendChild(makeHeaderCell('FLOW', 'label'));
    headRow.appendChild(makeHeaderCell('MAX Mbps', 'max'));
    thead.appendChild(headRow);

    const tbody = document.createElement('tbody');

    for (const stat of flowStats) {
      const i = stat.index;
      const row = document.createElement('tr');
      const colorCell = document.createElement('td');
      colorCell.style.backgroundColor = colors[(i - 1) % colors.length];
      colorCell.style.width = '20px';
      const labelCell = document.createElement('td');
      labelCell.textContent = stat.label;
      const maxCell = document.createElement('td');
      maxCell.textContent = stat.max.toFixed(1);
      row.appendChild(colorCell);
      row.appendChild(labelCell);
      row.appendChild(maxCell);
      tbody.appendChild(row);

      if (!window.seriesVisibility[i - 1]) {
        row.style.opacity = '0.4';
      } else {
        row.style.opacity = '1.0';
      }

      row.addEventListener('click', function(event) {
        const visibleCount = window.seriesVisibility.filter(Boolean).length;
        if (event.ctrlKey || event.metaKey || event.altKey) {
          if (window.seriesVisibility[i - 1] && visibleCount === 1) {
            window.seriesVisibility = Array(data[0].length - 1).fill(true);
          } else {
            window.seriesVisibility[i - 1] = !window.seriesVisibility[i - 1];
          }
        } else {
          if (window.seriesVisibility[i - 1] && visibleCount === 1) {
            window.seriesVisibility = Array(data[0].length - 1).fill(true);
          } else {
            window.seriesVisibility = Array(data[0].length - 1).fill(false);
            window.seriesVisibility[i - 1] = true;
          }
        }
        renderChart(rdata);
      });
    }

    table.appendChild(thead);
    table.appendChild(tbody);
    customLegendDiv.appendChild(table);
  }
}

function formatTimestamp(date) {
  return date.toISOString().substr(0, 16)
}

function loadValues(filterNum, field) {
    return $.getJSON("/dict_values/"+field, function(data) {
        $("#filter_value\\[" + filterNum + "\\]").autocomplete({
            source: data,
        });
    });
}
