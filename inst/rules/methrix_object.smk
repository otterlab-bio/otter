rule prepare_methrix_reference_cpg:
  message:"Prepare methrix reference CpG (.ron) ..."
  input:
    genome_ref = lambda wildcards: config["reference"]["files"]["fasta"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])],
    genome_annotation = lambda wildcards: config["reference"]["rnaseq"]["gtf"][config["workflow"]["species"]["name"].index(config["workflow"]["species"]["graft"])]
  output:
    os.path.join(config["directories"]["methylation_call"], "methrixh5", "reference_cpgs.ron")
  params:
    genome_key = config["workflow"]["species"]["graft"]
  shell:
    """
      mkdir -p "$(dirname {output})"
      ref="{input.genome_ref}"
      out_dir="$(dirname {output})"
      key="{params.genome_key}"

      annotation="{input.genome_annotation}"
      if [[ -L "$annotation" || ! -f "$annotation" ]]; then
        printf 'Methrix annotation must be a regular resolved GTF: %s\n' "$annotation" >&2
        exit 1
      fi
      install -m 0644 "$annotation" "$out_dir/$key.gtf"

      if [[ "$ref" == *.ron || "$ref" == *.con ]]; then
        cp -f "$ref" "{output}"
      elif [[ -f "${{ref}}.ron" ]]; then
        cp -f "${{ref}}.ron" "{output}"
      else
        stem="$(basename "$ref")"
        stem="${{stem%.gz}}"
        stem="${{stem%.fa}}"
        stem="${{stem%.fasta}}"
        stem="${{stem%.fna}}"
        ref_dir="$(dirname "$ref")"
        near_ron="${{ref_dir}}/${{stem}}.ron"

        if [[ -f "$near_ron" ]]; then
          cp -f "$near_ron" "{output}"
        else
          extract_command=(methx extract-cp-gs)
          if ! methx extract-cp-gs --help >/dev/null 2>&1; then
            extract_command=(methx extract-cpgs)
          fi
          "${{extract_command[@]}}" --genome "$ref" --output "{output}" || {{
            if [[ "${{extract_command[1]}}" == "extract-cp-gs" ]]; then
              extract_command=(methx extract-cpgs)
              "${{extract_command[@]}}" --genome "$ref" --output "{output}"
            else
              exit 1
            fi
          }}
          if grep -q 'cpgs: \[\]' "{output}"; then
            contig_arguments=()
            while IFS= read -r header; do
              if [[ "$header" == '>'* ]]; then
                contig="${{header#>}}"
                contig="${{contig%%[[:space:]]*}}"
                contig_arguments+=(--contigs "$contig")
              fi
            done < <(if [[ "$ref" == *.gz ]]; then gzip -cd "$ref"; else cat "$ref"; fi)
            if [[ "${{#contig_arguments[@]}}" -eq 0 ]]; then
              printf 'no FASTA contigs found in %s\n' "$ref" >&2
              exit 1
            fi
            "${{extract_command[@]}}" --genome "$ref" --output "{output}" "${{contig_arguments[@]}}"
          fi
        fi
      fi
    """

rule create_methrix_object :
  message:"Build beta matrix ..."
  input:
    expand(os.path.join(config["directories"]["methylation_call"], "{sample}_nsort.bismark.cov.gz"),sample = config["metadata"]["sample_ids"]),
    os.path.join(config["directories"]["methylation_call"], "methrixh5", "reference_cpgs.ron")
  output:
    os.path.join(config["directories"]["methylation_call"], "methrixh5","assays.h5"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","methrix_data.h5"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","CpG_coverage.xlsx"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","CpG_annotation_report.xlsx"),
    os.path.join(config["directories"]["methylation_call"], "methrixh5","CpG_annotation_details.tsv.gz")

  params:
    mcall_dir = config["directories"]["methylation_call"],
    methrix_dir = os.path.join(config["directories"]["methylation_call"], "methrixh5"),
    genome = os.path.join(config["directories"]["methylation_call"], "methrixh5", "reference_cpgs.ron")
  threads:10
  shell:
    """
      methx process \
        --input "{params.mcall_dir}" \
        --output "{params.methrix_dir}" \
        --genome "{params.genome}" \
        --annotation-dir "{params.methrix_dir}" \
        --threads {threads}

    """
   




    
