"use strict";

// const populationFile = "/static/ne_110m_admin_0_countries.geojson";
const populationFile = "/static/countries.geojson";
// const earthFile = "//unpkg.com/three-globe@2.21.4/example/img/earth-blue-marble.jpg"
// const earthFile = "//unpkg.com/three-globe@2.21.4/example/img/earth-night.jpg"
const earthFile = "//unpkg.com/three-globe@2.21.4/example/img/earth-dark.jpg"
const skyFile = '//unpkg.com/three-globe/example/img/night-sky.png'
const countriesFile = "/static/countries.json";
const csvTransferFile = "/static/country_transfer.csv";
const submarineCableFile = "//raw.githubusercontent.com/telegeography/www.submarinecablemap.com/master/web/public/api/v3/cable/cable-geo.json";

const polygonAltitudeStd = 0.01;
const polygonAltitudeHover = 0.03;
const ACR_COLOR_GRADIENT = [`rgba(0, 255, 0, 1)`, `rgba(255, 0, 0, 1)`]
const OPACITY_ARC_SELECTED = 1;
const OPACITY_ARC_UNSELECTED = 0.5;
const OPACITY_SELECTED = 0.98;
const OPACITY_STD = 0.8;
const OPACITY_UNSELECTED = 0.6;
var dtMultiplier = 1;
var paused = false;
function playpause() {
    if (paused) {
        dtMultiplier = 1
        paused = false;
        curTimeSld.disabled = true
        playbtn.innerText = "Pause"
    } else {
        dtMultiplier = 0
        paused = true;
        curTimeSld.disabled = false
        playbtn.innerText = "Play"
    }
}

function parseSubmarineCable(cablesGeo) {
    let cablePaths = [];
    cablesGeo.features.forEach(({ geometry, properties }) => {
        geometry.coordinates.forEach(coords => cablePaths.push({ coords, properties }));
    });
    return cablePaths;
}

function parseCountriesFile(values) {
    const countriesByCCA2 = new Map();
    const countriesByCCA3 = new Map();
    const latlngByCCA3 = new Map();
    const nameByCCA3 = new Map();

    values.forEach(country => {
        let name = country["name"]["common"];
        let cca2 = country["cca2"];
        let cca3 = country["cca3"];
        let latlng = country["latlng"];

        countriesByCCA2.set(cca2, country);
        countriesByCCA3.set(cca3, country);
        latlngByCCA3.set(cca3, latlng);
        nameByCCA3.set(cca3, name);

        if (cca3 === "UNK") {
            cca3 = "KSV"
            countriesByCCA3.set(cca3, country);
            latlngByCCA3.set(cca3, latlng);
            nameByCCA3.set(cca3, name);
        }

    });
    console.log("latlngByCCA3", latlngByCCA3)
    return { countriesByCCA2, countriesByCCA3, latlngByCCA3, nameByCCA3 };
}


let lastNet = null;


const COUNTRY = 'Portugal';

const MAP_CENTER = { lat: 50.3, lng: 14.9, altitude: 1.0 };

// add sun layer on top
const VELOCITY = 6; // minutes per frame

const sunPosAt = dt => {
    const day = new Date(+dt).setUTCHours(0, 0, 0, 0);
    const t = solar.century(dt);
    const longitude = (day - dt) / 864e5 * 360 - 180;
    return [longitude - solar.equationOfTime(t) / 4, solar.declination(t)];
};


var dt = (+(new Date()) - 24 * 3600 * 1000);
const solarTile = { pos: sunPosAt(dt) };
const timeEl = document.getElementById('time');
const playbtn = document.getElementById('playbtn');
const curTimeSld = document.getElementById('curTime');
const resetBtn = document.getElementById('reset');
const netList = document.getElementById('NetList');

function sliderChange() {
    dt = +curTimeSld.value;
}

function makeArcLabel(startCCA3, endCCA3, netStream, color) {
    return `<div style="background-color: rgba(0, 0, 0, 0.5); border-radius: 5px; style="color: ${color};">
    <b>${startCCA3} - ${endCCA3} :</b> <i style: "color: rgba(0, 255, 0, 0.9)" ><br />${netStream}</i>  MW
    </div>
    `
}

function loadFlowsToArcData(flows) {
    let arcs = flows.map((value) => {
        // { "border": "CTY|10YHR-HEP------M!CTY_CTY|10YHR-HEP------M_CTY_CTY|10YBA-JPCC-----D",
        //  "date": "30.01.2022", "starttimestr": "00:00", "endtimestr": "01:00", 
        //  "starttime": 1643497200, "endtime": 1643500800, "startLat": 44, "startLong": 18, "endLat": 45.166668,
        //   "endLong": 15.5, "startCCA3": "BIH", "endCCA3": "HRV", "timeframe": "00:00-01:00", 
        //   "upstreamDirection": ["BA", "HR"], "downstreamDirection": ["HR", "BA"], "upstream": 0, "downstream": 305, "netStream": -305 }
        let netStream = +value["netStream"]
        if (netStream > 0) {
            return {
                startLat: value["startLat"],
                startLng: value["startLong"],
                endLat: value["endLat"],
                endLng: value["endLong"],
                color: ACR_COLOR_GRADIENT,
                stroke: netStream / 4000,
                label: makeArcLabel(value["startCCA3"], value["endCCA3"], netStream, "rgba(255,255,255,1)")
            }
        } else {
            return {
                startLat: value["endLat"],
                startLng: value["endLong"],
                endLat: value["startLat"],
                endLng: value["startLong"],
                color: ACR_COLOR_GRADIENT,
                stroke: -netStream / 4000,
                label: makeArcLabel(value["startCCA3"], value["endCCA3"], -netStream, "rgba(255,255,255,1)")
            }
        }
    })
    return arcs
}

const k = 5.;
function sigmoid(z) {
    return 1. / (1. + Math.exp(-z / k));
}

function loadCountryValue(v) {
    let countryVal = getCountryVal(v)
    if (countryVal === null || countryVal === undefined) {
        return 0
    }
    return sigmoid(value / 1000.) / 50 + 0.001;
}

function loadPointsPerCountry(valueByCountry, latlngByCCA3) {
    let results = []

    for (const [cca3, value] of valueByCountry.entries()) {
        if (latlngByCCA3.has(cca3)) {
            results.push(
                {
                    lat: latlngByCCA3.get(cca3)[0],
                    lng: latlngByCCA3.get(cca3)[1],
                    size: Math.sqrt(Math.abs(value) / 4000000),
                    radius: Math.sqrt(Math.abs(value) / 100000),
                    color: value > 0 ? 'rgba(0, 255, 0, 0.85)' : 'rgba(255, 0, 0, 0.85)'
                })
        }

    }
    return results
}

function getCountryCodeByProperties(v) {
    let countrycode = "-99";

    if (!v || !v.properties) {
        return null;
    } else if (v.properties.ISO_A3 !== "-99") {
        countrycode = v.properties.ISO_A3;
    } else if (v.properties.WB_A3 !== "-99") {
        countrycode = v.properties.WB_A3;
    } else if (v.properties.SU_A3 !== "-99") {
        countrycode = v.properties.SU_A3;
    }
    return countrycode;
}

function getCountryVal(valueByCountry, v) {
    let countrycode = getCountryCodeByProperties(v);

    if (countrycode === "-99" || (!valueByCountry.has(countrycode))) {
        return null;
    } else {
        return valueByCountry.get(countrycode) / 1000.
    }

}

function loadCountryColorWithOpacity(valueByCountry, v, opacity) {
    let countryVal = getCountryVal(valueByCountry, v)
    if (countryVal === null || countryVal === undefined) {
        return 'rgba(0, 0, 0, ' + OPACITY_UNSELECTED + ')'
    }
    let val = sigmoid(countryVal)
    let color = 'rgba(' + Math.round(255 * (1 - val)) + ',' + Math.round(255 * val) + ', 0,' + opacity + ')';
    return color;
}



function onDataLoaded([{ countriesByCCA2, countriesByCCA3, latlngByCCA3, nameByCCA3 }, countries, { Flows, Net, CountryInfo }]) {

    const minTime = Math.min(...Object.keys(Net)) * 1000
    const maxTime = Math.max(...Object.keys(Net)) * 1000

    curTimeSld.max = maxTime
    curTimeSld.min = minTime

    // // const colorScale = d3.scale.linear().range(["red", "white", "green"])
    // const interpolate = d3.interpolateRgb("red", "white", "green");
    // const colorScale = d3.scaleSequentialSqrt(interpolate);
    // // const colorScale = d3.scaleSequentialSqrt(d3.interpolateYlOrRd);
    // colorScale.domain([-1e2, 1e2]);


    const d3colorScale = d3.scaleSqrt()
        .domain([-1, 0, 1])
        .range(["red", "yellow", "green"]);

    const colorScale = (v) => d3colorScale(sigmoid(v) - 0.5);

    function countryToColor(entriesMap, v) {
        let val = getCountryVal(entriesMap, v);
        if (val !== null && val !== undefined) {
            return colorScale(val)
        } else {
            return 'black'
        }
    }

    function updateNetList(valueByCountry) {
        console.log("updateNetList", valueByCountry);
        netList.innerHTML = ""
        for (let [cca3, value] of valueByCountry.entries()) {
            let color = colorScale(value / 1000)
            let newLi = document.createElement("li");
            newLi.innerText = cca3 + " " + Math.round(value) + " MW";
            newLi.style = "color: " + color;
            netList.appendChild(newLi);
        }
    }

    let lastFlow = null;
    const FlowAt = (dt) => {
        let unixSeconds = Math.round(dt / 1000 / 3600) * 3600
        if (unixSeconds !== lastFlow && unixSeconds in Flows) {
            globe
                .arcsData(loadFlowsToArcData(Flows[unixSeconds]))
                .onArcHover(hoverArc => globe
                    .arcColor(d => {
                        const op = !hoverArc ? OPACITY_ARC_UNSELECTED : d === hoverArc ? OPACITY_ARC_SELECTED : OPACITY_ARC_UNSELECTED / 4;
                        return [`rgba(0, 255, 0, ${op})`, `rgba(255, 0, 0, ${op})`];
                    })
                );
            lastFlow = unixSeconds;
        }
    }


    const NetAt = (dt) => {
        let unixSeconds = "" + Math.round(dt / 1000 / 3600) * 3600
        if (lastNet != unixSeconds && unixSeconds in Net) {
            let entriesMap = new Map(Object.entries(Net[unixSeconds]));
            const maxVal = Math.max(...entriesMap.values());
            const minVal = Math.min(...entriesMap.values());
            console.log("Scale: ", maxVal, minVal, entriesMap)
            globe
                // .polygonCapColor((v) => loadCountryColorWithOpacity(entriesMap, v, OPACITY_STD))
                .polygonCapColor((v) => countryToColor(entriesMap, v))
                .polygonAltitude(0.01)
                .polygonLabel((v) => `
                <div style="background-color: rgba(0, 0, 0, 0.5); border-radius: 5px; color: rgba(255, 255, 255, 1);"><b>${getCountryCodeByProperties(v)}:</b> <br />
                <i> Net: ${getCountryVal(entriesMap, v)}</i> GW <br/></div>`)
                .onPolygonHover(hoverD => {
                    return globe
                        .polygonAltitude(d => d === hoverD ? polygonAltitudeHover : polygonAltitudeStd)
                        // .polygonCapColor(d => d === hoverD ? loadCountryColorWithOpacity(entriesMap, d, OPACITY_SELECTED): loadCountryColorWithOpacity(entriesMap, d, OPACITY_STD))
                        .polygonCapColor(d => d === hoverD ? countryToColor(entriesMap, d) : countryToColor(entriesMap, d))
                }
                )
                .lineHoverPrecision(0)


            // add bins displying countries energy usage
            globe.pointsData(loadPointsPerCountry(entriesMap, latlngByCCA3))
                .pointAltitude('size')
                .pointRadius('radius')
                .pointColor('color')
            lastNet = unixSeconds;
            updateNetList(entriesMap);
        }
    }

    const globe = new Globe()
        (document.getElementById('globeViz'))
        .globeImageUrl(earthFile)
        .backgroundImageUrl(skyFile)
        .tilesData([solarTile])
        .tileLng(d => d.pos[0])
        .tileLat(d => d.pos[1])
        .tileAltitude(0.005)
        .tileWidth(180)
        .tileHeight(180)
        .tileUseGlobeProjection(false)
        .tileMaterial(() => new THREE.MeshLambertMaterial({ color: '#ffff00', opacity: 0.1, transparent: true }))
        .tilesTransitionDuration(0)

        .polygonsData(countries.features.filter(d => d.properties.code !== 'AQ'))
        .polygonSideColor(() => 'rgba(0, 0, 0, 1)')
        .polygonStrokeColor(() => '#111')
        .polygonAltitude(0.01)
        .polygonsTransitionDuration(1000)

        // load submarine cables with labels
        // .pathsData(cablesGeo)
        // .pathPoints('coords')
        // .pathPointLat(p => p[1])
        // .pathPointLng(p => p[0])
        // .pathColor(path => path.properties.color)
        // .pathLabel(path => path.properties.name)
        // .pathDashLength(0.1)
        // .pathDashGap(0.008)
        // .pathDashAnimateTime(12000)

        // add lines transporting energy between countries
        .arcsData([])
        .arcColor('color')
        .arcStroke('stroke')
        .arcLabel('label')
        .arcDashLength(0.4)
        .arcAltitude(0.07)
        .arcDashGap(0.01)
        .arcDashInitialGap(() => 0.01)
        .arcDashAnimateTime(4000)
        .arcsTransitionDuration(0);

    globe
        .pointOfView(MAP_CENTER, 4000);



    resetBtn.addEventListener("click", () => globe
        .pointOfView(MAP_CENTER, 4000));

    // animate time of day
    requestAnimationFrame(() =>
        (function animate() {
            dt += VELOCITY * 60 * 10 * dtMultiplier;
            if (dt > maxTime) {
                dt = minTime
            } else if (dt < minTime) {
                dt = minTime
            }
            FlowAt(dt)
            NetAt(dt)
            solarTile.pos = sunPosAt(dt);
            globe.tilesData([solarTile]);
            timeEl.textContent = new Date(dt).toLocaleString();
            curTimeSld.value = dt
            requestAnimationFrame(animate);
        })()
    );

}

Promise.all([
    fetch(countriesFile).then(r => r.json()).then(countries => parseCountriesFile(countries)),
    fetch(populationFile).then(r => r.json()).then(population => population),
    fetch("/api/total").then(r => r.json()).then(r => r.data)
]).then(([{ countriesByCCA2, countriesByCCA3, latlngByCCA3, nameByCCA3 }, countries, { Flows, Net, CountryInfo }]) =>
    onDataLoaded([{ countriesByCCA2, countriesByCCA3, latlngByCCA3, nameByCCA3 }, countries, { Flows, Net, CountryInfo }])
);