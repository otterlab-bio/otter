import json
import os


rule rnaseq_splicing:
    message: "RNA Splicing ..."
    input:
        expand(os.path.join(config["directories"]["bsmap"]["main"], "{sample}_{species}.bam"), sample=config["metadata"]["sample_ids"], species=config["workflow"]["species"]["name"])
    output:
        outcome=os.path.join(config["directories"]["bsmap"]["main"], "RNASplicing", "splicing-outcome.json")
    params:
        run_dir=config["directories"]["bsmap"]["main"],
        pdata=os.path.join(config["directories"]["selfconfig"], "pdata.xlsx"),
        seqlengthQC=config["directories"]["qc"]["main"],
        gtf=config["reference"]["rnaseq"]["gtf"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])],
        splicing_dir=os.path.join(config["directories"]["bsmap"]["main"], "RNASplicing"),
        pdxmode=(0 if not config["metadata"]["pdx_pipeline"] else 1)
    threads: 20
    run:
        os.makedirs(params.splicing_dir, exist_ok=True)
        if config["metadata"]["group_levels"] >= 2:
            shell(
                """
                enva run base \
                  Rscript R/RNA_Splicing.R \
                  --root {params.run_dir:q} \
                  --threads {threads} \
                  --pdata {params.pdata:q} \
                  --seqlengthQC {params.seqlengthQC:q} \
                  --gtf {params.gtf:q} \
                  --pdxmode {params.pdxmode}
                """
            )
            artifact_paths = sorted(
                os.path.relpath(os.path.join(directory_path, filename), params.splicing_dir)
                for directory_path, _, filenames in os.walk(params.splicing_dir)
                for filename in filenames
                if os.path.join(directory_path, filename) != output.outcome
            )
            if not artifact_paths:
                raise ValueError("legacy RNA splicing completed without producing artifacts")
            outcome = {
                "schema_version": "otter.rna-splicing-outcome/v1",
                "status": "produced",
                "source_root": os.path.abspath(params.splicing_dir),
                "artifact_paths": artifact_paths,
            }
        else:
            outcome = {
                "schema_version": "otter.rna-splicing-outcome/v1",
                "status": "not_applicable",
                "source_root": os.path.abspath(params.splicing_dir),
                "artifact_paths": [],
            }
        with open(output.outcome, "w", encoding="utf-8") as outcome_file:
            json.dump(outcome, outcome_file, indent=2)
            outcome_file.write("\n")
