import json
import os


def rnaseq_splicing_bams(wildcards):
    pdx_mode = str(config["metadata"]["pdx_pipeline"]).strip().lower() in {"1", "true", "yes"}
    if pdx_mode:
        return expand(
            os.path.join(config["directories"]["bsmap"]["main"], "Filtered_bams", "{sample}_fixed_" + config["workflow"]["species"]["graft"] + "_Filtered.bam"),
            sample=config["metadata"]["sample_ids"],
        )
    return expand(
        os.path.join(config["directories"]["bsmap"]["main"], "{sample}_{species}.bam"),
        sample=config["metadata"]["sample_ids"],
        species=config["workflow"]["species"]["name"],
    )


rule rnaseq_splicing:
    message: "RNA Splicing ..."
    input:
        bams=rnaseq_splicing_bams,
        pdata=os.path.join(config["directories"]["selfconfig"], "pdata.xlsx"),
        gtf=config["reference"]["rnaseq"]["gtf"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])]
    output:
        outcome=os.path.join(config["directories"]["bsmap"]["main"], "RNASplicing", "splicing-outcome.json")
    params:
        run_dir=config["directories"]["bsmap"]["main"],
        seqlengthQC=config["directories"]["qc"]["main"],
        splicing_dir=os.path.join(config["directories"]["bsmap"]["main"], "RNASplicing"),
        pdxmode=(1 if str(config["metadata"]["pdx_pipeline"]).strip().lower() in {"1", "true", "yes"} else 0)
    threads: 20
    run:
        os.makedirs(params.splicing_dir, exist_ok=True)
        if config["metadata"]["group_levels"] >= 2:
            shell(
                """
                enva run otter-core -- \
                  matsrun run \
                  --root {params.run_dir:q} \
                  --threads {threads} \
                  --pdata {input.pdata:q} \
                  --seqlengthQC {params.seqlengthQC:q} \
                  --gtf {input.gtf:q} \
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
                raise ValueError("matsrun reported success but produced no splicing artifacts")
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
