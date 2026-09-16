rule count_insert:
    message: "count insert ..."
    input:
        sample_bam = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_{wildcards.species}.bam")
    output:
        os.path.join(config["directories"]["qc"]["main"], "{sample}_{species}_insert_length.txt")
    params:
        insert_length = lambda wildcards:os.path.join(config["directories"]["qc"]["main"], f"{wildcards.sample}_{wildcards.species}_insert_length.txt")
    threads: 16
    shell:
        """
        chmod +x R/count_insert.sh
        bash R/count_insert.sh {input.sample_bam} {params.insert_length}
        """
