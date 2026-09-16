rule picard_pdx_patch:
  message: "Patch legacy PDX BAM tags"
  input:
    bam_sorted=lambda wildcards: os.path.join(
      config["directories"]["bsmap"]["main"],
      f"{wildcards.sample}_{wildcards.species}.bam",
    )
  output:
    fixed_bam=os.path.join(
      config["directories"]["bsmap"]["main"],
      "{sample}_fixed_{species}.bam",
    )
  params:
    fasta=lambda wildcards: config["reference"]["files"]["fasta"][
      config["workflow"]["species"]["name"].index(wildcards.species)
    ],
    is_bisulfite=("false" if config["mode"] == "RNASEQ" else "true")
  threads: 4
  shell:
    """
    set -euo pipefail
    enva run picard -- picard SetNmMdAndUqTags \
      I={input.bam_sorted:q} \
      O={output.fixed_bam:q} \
      R={params.fasta:q} \
      IS_BISULFITE_SEQUENCE={params.is_bisulfite}
    test ! -L {output.fixed_bam:q}
    test -s {output.fixed_bam:q}
    """
