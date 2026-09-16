rule mhap_analysis:
    message: "mhap ..."
    input:
        sample_bam = lambda wildcards: os.path.join(config["directories"]["bsmap"]["main"], f"{wildcards.sample}_{wildcards.species}.bam")
    output:
        mapgz = os.path.join(config["directories"]["mhap"], "{sample}_{species}.mhap.gz"),
        cgi_summary = os.path.join(config["directories"]["mhap"], "{sample}_{species}_CGI_summary.txt")
    params:
        mapgz = lambda wildcards:os.path.join(config["directories"]["mhap"], f"{wildcards.sample}_{wildcards.species}.mhap.gz"),
        cgi_summary = lambda wildcards:os.path.join(config["directories"]["mhap"], f"{wildcards.sample}_{wildcards.species}_CGI_summary.txt"),
        cpg = config["reference"]["files"]["cpg_sites"],
        cgi = config["reference"]["files"]["cgi"]
    threads: 4
    shell:
        """
        export LD_LIBRARY_PATH=/mHapTools/htslib-1.10.2/lib
        mhaptools convert -i {input.sample_bam} -c {params.cpg} -o {params.mapgz}
        tabix -b 2 -e 3 -p bed {params.mapgz}
        mhaptools summary -i {params.mapgz} -c {params.cpg} -b {params.cgi} -o {params.cgi_summary}
        """
