import pkg from 'pg';
const { Pool } = pkg;

const pool = new Pool({
  user: 'admin',
  host: 'localhost',
  database: 'basyx',
  password: 'admin123',
  port: 5432
});

/**
 * Remove null/undefined fields from an object
 */
function clean(obj) {
  if (Array.isArray(obj)) {
    return obj.map(clean).filter(v => v !== null && v !== undefined);
  } else if (obj && typeof obj === 'object') {
    const out = {};
    for (const [k, v] of Object.entries(obj)) {
      if (v !== null && v !== undefined) {
        out[k] = clean(v);
      }
    }
    return out;
  }
  return obj;
}

async function fetchSubmodel(submodelId) {
  const client = await pool.connect();
  try {
    const { rows: [submodel] } = await client.query(
      `SELECT id, id_short, category, kind
       FROM submodel
       WHERE id = $1`, [submodelId]
    );
    if (!submodel) throw new Error(`Submodel ${submodelId} not found`);

    // load all SMEs with their possible values
    const { rows: elements } = await client.query(
      `SELECT e.id, e.parent_sme_id, e.position, e.id_short, e.model_type,
              p.value_type AS prop_type,
              COALESCE(p.value_text,
                       p.value_num::text,
                       p.value_bool::text,
                       p.value_time::text,
                       p.value_datetime::text) AS prop_value
       FROM submodel_element e
       LEFT JOIN property_element p ON p.id = e.id
       WHERE e.submodel_id = $1
       ORDER BY e.parent_sme_id NULLS FIRST, e.position, e.id`, [submodelId]
    );

    // Map id → element
    const elementMap = {};
    for (const row of elements) {
      if (!elementMap[row.id]) {
        const elem = {
          id: row.id.toString(),
          idShort: row.id_short,
          modelType: row.model_type
        };

        if (row.model_type === 'Property') {
          elem.valueType = row.prop_type;
          elem.value = row.prop_value;
        }
        if (row.model_type === 'File') {
          elem.contentType = row.file_type;
          elem.value = row.file_value;
        }
        if (row.model_type === 'MultiLanguageProperty') {
          elem.value = [];
        }
        if (row.model_type === 'SubmodelElementCollection') {
          elem.value = [];
        }
        if (row.model_type === 'SubmodelElementList') {
          elem.value = [];
        }

        elementMap[row.id] = elem;
        elementMap[row.id]._parentId = row.parent_sme_id; // keep for tree building
      }

      // collect multilanguage values
      if (row.model_type === 'MultiLanguageProperty' && row.ml_lang) {
        elementMap[row.id].value.push({
          language: row.ml_lang,
          text: row.ml_text
        });
      }
    }

    // build tree
    const roots = [];
    for (const elem of Object.values(elementMap)) {
      if (elem._parentId) {
        const parent = elementMap[elem._parentId];
        if (parent && Array.isArray(parent.value)) {
          if(parent.modelType === 'SubmodelElementList') {
            delete elem.idShort; // lists do not have idShorts
            delete elem.position; // lists do not have positions
          }
          delete elem.id;
          parent.value.push(elem);
        }
      } else {
        roots.push(elem);
      }
      delete elem._parentId; // remove helper field
    }

    // final object, cleaned
    return clean({
      id: submodel.id,
      idShort: submodel.id_short,
      category: submodel.category, // will be removed if null
      kind: submodel.kind,
      submodelElements: roots
    });
  } finally {
    client.release();
  }
}

// run as script
(async () => {
  try {
    const submodel = await fetchSubmodel('http://iese.fraunhofer.de/id/sm/DemoSubmodel');
    console.log(JSON.stringify(submodel, null, 2));
    const submodel2 = await fetchSubmodel('sm-42');
    console.log(JSON.stringify(submodel2, null, 2));
    await pool.end();
  } catch (err) {
    console.error('Error:', err);
    process.exit(1);
  }
})();
